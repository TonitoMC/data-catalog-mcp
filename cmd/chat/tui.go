package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/tonitomc/data-catalog-mcp/internal/client"
	"github.com/tonitomc/data-catalog-mcp/internal/llm"
)

// Minimalist, low-chrome palette: a single accent color for structure
// (prompt, borders, the model's own turns), everything else falls back to
// dim/error accents. Fixed (not lipgloss.AdaptiveColor) deliberately —
// AdaptiveColor queries the terminal's background via an OSC escape
// sequence to pick light vs. dark, and that query races Bubbletea's own
// stdin reader for keyboard input. On terminals where that race goes
// wrong the query never resolves, freezing the whole program and leaking
// the unread response bytes to the terminal once it's killed.
var (
	colorAccent = lipgloss.Color("39")
	colorDim    = lipgloss.Color("243")
	colorError  = lipgloss.Color("203")

	userStyle      = lipgloss.NewStyle().Bold(true)
	assistantStyle = lipgloss.NewStyle().Foreground(colorAccent)
	toolStyle      = lipgloss.NewStyle().Foreground(colorDim)
	errorStyle     = lipgloss.NewStyle().Foreground(colorError)
	statusStyle    = lipgloss.NewStyle().Foreground(colorDim)
	headerStyle    = lipgloss.NewStyle().Foreground(colorDim)
	promptStyle    = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
)

// chatModel is the Bubble Tea model driving the REPL. The LLM/router
// plumbing is unchanged from the plain CLI version — this only replaces
// how input/output are rendered.
type chatModel struct {
	program *tea.Program // set right after construction, used to stream tool-call/answer updates mid-turn

	llmClient llm.Client
	router    *client.Router
	tools     []llm.Tool
	resolver  *toolResolver

	history []llm.Message // full conversation sent to the model
	entries []string       // rendered lines, one per event (user turn, tool call, answer, error)

	viewport viewport.Model
	input    textinput.Model
	spinner  spinner.Model

	busy   bool
	width  int
	height int

	header string
}

func newChatModel(llmClient llm.Client, router *client.Router, tools []llm.Tool, resolver *toolResolver, header string) *chatModel {
	ti := textinput.New()
	ti.Placeholder = "Ask something..."
	ti.Focus()
	ti.Prompt = promptStyle.Render("> ")
	ti.CharLimit = 4000

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = toolStyle

	vp := viewport.New(80, 20)

	return &chatModel{
		llmClient: llmClient,
		router:    router,
		tools:     tools,
		resolver:  resolver,
		history:   []llm.Message{{Role: "system", Content: systemPrompt}},
		viewport:  vp,
		input:     ti,
		spinner:   sp,
		header:    header,
	}
}

func (m *chatModel) Init() tea.Cmd {
	return textinput.Blink
}

// Messages sent back from the goroutine running a turn (see runTurnCmd),
// so tool calls appear as they happen instead of all at once at the end.
type toolCallDoneMsg struct {
	name   string
	result string
}
type turnDoneMsg struct{ history []llm.Message }
type turnErrMsg struct{ err error }

func (m *chatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.viewport.Width = max(msg.Width, 1)
		m.viewport.Height = max(msg.Height-inputAreaHeight(m.header), 1)
		m.input.Width = max(msg.Width-lipgloss.Width(m.input.Prompt)-1, 1)
		m.renderViewport()
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.busy {
				return m, nil
			}
			text := strings.TrimSpace(m.input.Value())
			if text == "" {
				return m, nil
			}
			if text == "/exit" {
				return m, tea.Quit
			}
			return m.submit(text)
		}

	case toolCallDoneMsg:
		m.entries = append(m.entries, toolStyle.Render(fmt.Sprintf("  ↳ %s → %s", msg.name, truncate(msg.result, 200))))
		m.renderViewport()
		return m, nil

	case assistantMsg:
		m.handleAssistant(msg)
		return m, nil

	case turnDoneMsg:
		m.history = msg.history
		m.busy = false
		m.input.Focus()
		m.renderViewport()
		return m, nil

	case turnErrMsg:
		m.entries = append(m.entries, errorStyle.Render("error: "+msg.err.Error()))
		m.busy = false
		m.input.Focus()
		m.renderViewport()
		return m, nil

	case spinner.TickMsg:
		if m.busy {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

// submit appends the user's message, locks input, and kicks off the
// tool-call loop as a background command.
func (m *chatModel) submit(text string) (tea.Model, tea.Cmd) {
	m.entries = append(m.entries, userStyle.Render("> ")+text)
	m.history = append(m.history, llm.Message{Role: "user", Content: text})
	m.input.SetValue("")
	m.input.Blur()
	m.busy = true
	m.renderViewport()
	return m, tea.Batch(m.spinner.Tick, m.runTurnCmd())
}

// runTurnCmd drives the tool-call loop for the latest user message: ask
// the model, execute any tool calls via the router, feed results back,
// repeat until the model answers with plain content. It streams each
// step back through m.program.Send so the UI updates as it goes, rather
// than freezing until the whole turn is done.
func (m *chatModel) runTurnCmd() tea.Cmd {
	history := append([]llm.Message(nil), m.history...)
	return func() tea.Msg {
		ctx := context.Background()
		for {
			reply, err := m.llmClient.Chat(ctx, history, m.tools)
			if err != nil {
				return turnErrMsg{err: fmt.Errorf("talking to model: %w", err)}
			}
			history = append(history, reply)

			if len(reply.ToolCalls) == 0 {
				m.program.Send(assistantMsg{content: reply.Content})
				return turnDoneMsg{history: history}
			}

			for _, tc := range reply.ToolCalls {
				result := runToolCall(tc, m.resolver, m.router)
				m.program.Send(toolCallDoneMsg{name: tc.Function.Name, result: result})
				history = append(history, llm.Message{Role: "tool", Content: result, ToolCallID: tc.ID})
			}
		}
	}
}

// assistantMsg carries the model's final answer for one turn, sent
// mid-command (see runTurnCmd) so it renders before turnDoneMsg lands.
type assistantMsg struct{ content string }

func (m *chatModel) View() string {
	var b strings.Builder
	if m.header != "" {
		b.WriteString(headerStyle.Render(m.header))
		b.WriteString("\n")
	}
	b.WriteString(m.viewport.View())
	b.WriteString("\n")
	if m.busy {
		b.WriteString(m.spinner.View() + " " + statusStyle.Render("thinking..."))
	} else {
		b.WriteString(m.input.View())
	}
	return b.String()
}

func inputAreaHeight(header string) int {
	h := 2 // blank-ish line + input line
	if header != "" {
		h++
	}
	return h
}

func (m *chatModel) renderViewport() {
	m.viewport.SetContent(strings.Join(m.entries, "\n\n"))
	m.viewport.GotoBottom()
}

// handleAssistantMsg is wired into Update via the default case's message
// routing below — kept separate for readability.
func (m *chatModel) handleAssistant(msg assistantMsg) {
	m.entries = append(m.entries, renderMarkdown(msg.content, m.viewport.Width))
	m.renderViewport()
}

// renderMarkdown renders the model's answer as markdown (headings, lists,
// code blocks, bold/italic, ...) to ANSI, wrapped to width. Falls back to
// plain accent-colored text if rendering fails for any reason — a
// malformed render shouldn't ever hide the model's answer.
func renderMarkdown(content string, width int) string {
	if width <= 0 {
		width = 80
	}
	r, err := glamour.NewTermRenderer(
		// Fixed style, not WithAutoStyle: auto-style also queries the
		// terminal background over the same OSC mechanism that races
		// Bubbletea's stdin reader (see colorAccent above) — avoid it here
		// too rather than trading one hang for another.
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return assistantStyle.Render(content)
	}
	out, err := r.Render(content)
	if err != nil {
		return assistantStyle.Render(content)
	}
	return strings.TrimRight(out, "\n")
}
