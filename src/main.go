// GoEdit v2.0
// Copyright © Prof. Dr. Michael Stal, 2025
// All rights reserved.

package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "time"

    "github.com/gdamore/tcell/v2"
)

const version = "2.0.0"

type Editor struct {
    screen         tcell.Screen
    tabManager     *TabManager
    clipboard      *ClipboardManager
    width          int
    height         int
    statusMsg      string
    statusMsgMutex sync.RWMutex
    mode           EditorMode
    findQuery      string
    llmClient      *OllamaClient
    llmPrompt      string
    llmResponse    string
    llmMutex       sync.RWMutex
    inputBuffer    string
    quitAttempts   int
    aiMutex        sync.Mutex
    aiInProgress   bool
    aiCancel       chan bool
    streamEnabled  bool
}

type EditorMode int

const (
    ModeNormal EditorMode = iota
    ModeFind
    ModeGoto
    ModeLLM
    ModeFilename
)

func NewEditor(filenames []string, ollamaURL, model string, streamEnabled bool) (*Editor, error) {
    screen, err := tcell.NewScreen()
    if err != nil {
        return nil, fmt.Errorf("failed to create screen: %w", err)
    }

    if err := screen.Init(); err != nil {
        return nil, fmt.Errorf("failed to initialize screen: %w", err)
    }

    screen.EnableMouse()
    screen.EnablePaste()
    screen.Clear()

    width, height := screen.Size()
    if height < 5 {
        screen.Fini()
        return nil, fmt.Errorf("terminal too small (need at least 5 lines)")
    }

    tabManager := NewTabManager()
    
    if len(filenames) == 0 {
        if err := tabManager.AddTab(""); err != nil {
            screen.Fini()
            return nil, fmt.Errorf("failed to create initial tab: %w", err)
        }
    } else {
        for _, filename := range filenames {
            if err := tabManager.AddTab(filename); err != nil {
                screen.Fini()
                return nil, fmt.Errorf("failed to open %s: %w", filename, err)
            }
        }
    }

    return &Editor{
        screen:        screen,
        tabManager:    tabManager,
        clipboard:     NewClipboardManager(),
        width:         width,
        height:        height - 3,
        statusMsg:     "Ctrl+Q: Quit | Ctrl+S: Save | Ctrl+L: AI | Ctrl+T: New Tab | Tab: Next",
        mode:          ModeNormal,
        llmClient:     NewOllamaClient(ollamaURL, model),
        aiInProgress:  false,
        aiCancel:      make(chan bool, 1),
        streamEnabled: streamEnabled,
    }, nil
}

func (e *Editor) setStatusMsg(msg string) {
    e.statusMsgMutex.Lock()
    e.statusMsg = msg
    e.statusMsgMutex.Unlock()
}

func (e *Editor) getStatusMsg() string {
    e.statusMsgMutex.RLock()
    defer e.statusMsgMutex.RUnlock()
    return e.statusMsg
}

func (e *Editor) setLLMResponse(response string) {
    e.llmMutex.Lock()
    e.llmResponse = response
    e.llmMutex.Unlock()
}

func (e *Editor) getLLMResponse() string {
    e.llmMutex.RLock()
    defer e.llmMutex.RUnlock()
    return e.llmResponse
}

func (e *Editor) appendLLMResponse(chunk string) {
    e.llmMutex.Lock()
    e.llmResponse += chunk
    e.llmMutex.Unlock()
}

func (e *Editor) checkOllamaSetup() error {
    if !e.llmClient.IsAvailable() {
        return fmt.Errorf("Ollama not running. Start with: ollama serve")
    }
    
    if err := e.llmClient.CheckModel(); err != nil {
        return err
    }
    
    return nil
}

func (e *Editor) Run() error {
    if e.screen == nil {
        return fmt.Errorf("screen not initialized")
    }

    defer e.screen.Fini()

    e.render()

    for {
        ev := e.screen.PollEvent()
        if ev == nil {
            continue
        }

        if !e.handleEvent(ev) {
            return nil
        }

        e.render()
    }
}

func (e *Editor) handleEvent(ev tcell.Event) bool {
    switch ev := ev.(type) {
    case *tcell.EventResize:
        e.width, e.height = ev.Size()
        if e.height < 5 {
            e.height = 5
        }
        e.height -= 3
        e.screen.Sync()
        return true

    case *tcell.EventKey:
        if ev.Key() == tcell.KeyEscape {
            e.aiMutex.Lock()
            if e.aiInProgress {
                select {
                case e.aiCancel <- true:
                default:
                }
                e.aiInProgress = false
                e.setStatusMsg("AI request cancelled")
                e.aiMutex.Unlock()
                if e.mode == ModeLLM {
                    e.mode = ModeNormal
                }
                return true
            }
            e.aiMutex.Unlock()
        }

        return e.handleKey(ev)
    }

    return true
}

func (e *Editor) handleKey(ev *tcell.EventKey) bool {
    if ev == nil {
        return true
    }

    switch e.mode {
    case ModeFind:
        return e.handleFindMode(ev)
    case ModeGoto:
        return e.handleGotoMode(ev)
    case ModeLLM:
        return e.handleLLMMode(ev)
    case ModeFilename:
        return e.handleFilenameMode(ev)
    default:
        return e.handleNormalMode(ev)
    }
}

func (e *Editor) handleNormalMode(ev *tcell.EventKey) bool {
    tab := e.tabManager.GetActiveTab()
    if tab == nil || tab.buffer == nil || tab.cursor == nil {
        return true
    }

    mod := ev.Modifiers()

    switch ev.Key() {
    case tcell.KeyCtrlQ:
        e.aiMutex.Lock()
        inProgress := e.aiInProgress
        e.aiMutex.Unlock()

        if inProgress {
            e.setStatusMsg("AI in progress. Press Esc to cancel, then Ctrl+Q to quit")
            return true
        }

        if tab.buffer.modified && e.quitAttempts == 0 {
            e.setStatusMsg("File modified! Press Ctrl+Q again to quit, Ctrl+S to save")
            e.quitAttempts++
            return true
        }
        
        for i := 0; i < e.tabManager.GetTabCount(); i++ {
            t := e.tabManager.tabs[i]
            if t != nil && t.buffer != nil && t.buffer.modified {
                e.setStatusMsg(fmt.Sprintf("Tab %d has unsaved changes! Save all or press Ctrl+Q again", i+1))
                if e.quitAttempts == 0 {
                    e.quitAttempts++
                    return true
                }
            }
        }
        
        return false

    case tcell.KeyCtrlS:
        e.quitAttempts = 0
        e.saveFile()

    case tcell.KeyCtrlT:
        if err := e.tabManager.AddTab(""); err != nil {
            e.setStatusMsg(fmt.Sprintf("Failed to create new tab: %v", err))
        } else {
            e.setStatusMsg(fmt.Sprintf("New tab created (Tab %d)", e.tabManager.GetTabCount()))
        }

    case tcell.KeyCtrlW:
        if e.tabManager.CloseTab() {
            e.setStatusMsg("Cannot close last tab")
        } else {
            e.setStatusMsg(fmt.Sprintf("Tab closed (now at Tab %d)", e.tabManager.activeTab+1))
        }

    case tcell.KeyTab:
        if mod&tcell.ModShift != 0 {
            e.tabManager.PrevTab()
        } else {
            e.tabManager.NextTab()
        }
        newTab := e.tabManager.GetActiveTab()
        if newTab != nil && newTab.buffer != nil {
            filename := newTab.buffer.filename
            if filename == "" {
                filename = "[No Name]"
            } else {
                filename = filepath.Base(filename)
            }
            e.setStatusMsg(fmt.Sprintf("Switched to: %s (Tab %d/%d)", 
                filename, e.tabManager.activeTab+1, e.tabManager.GetTabCount()))
        }

    case tcell.KeyCtrlF:
        e.mode = ModeFind
        e.inputBuffer = ""
        e.setStatusMsg("Find: ")

    case tcell.KeyCtrlG:
        e.mode = ModeGoto
        e.inputBuffer = ""
        e.setStatusMsg("Go to line: ")

    case tcell.KeyCtrlL:
        if err := e.checkOllamaSetup(); err != nil {
            e.setStatusMsg(fmt.Sprintf("AI unavailable: %v", err))
            return true
        }
        e.mode = ModeLLM
        e.inputBuffer = ""
        if e.streamEnabled {
            e.setStatusMsg("Ask AI (streaming): ")
        } else {
            e.setStatusMsg("Ask AI: ")
        }

    case tcell.KeyCtrlK:
        response := e.getLLMResponse()
        if response != "" {
            oldRow := tab.cursor.Row
            oldCol := tab.cursor.Col
            tab.buffer.InsertText(tab.cursor.Row, tab.cursor.Col, response)

            lines := strings.Split(response, "\n")
            if len(lines) > 1 {
                tab.cursor.Row = oldRow + len(lines) - 1
                if tab.cursor.Row < 0 {
                    tab.cursor.Row = 0
                }
                if len(lines) > 0 {
                    lastLine := lines[len(lines)-1]
                    tab.cursor.Col = len(lastLine)
                }
            } else {
                tab.cursor.Col = oldCol + len(response)
            }

            e.ensureCursorValid(tab)
            tab.buffer.SaveState(tab.cursor.Row, tab.cursor.Col)
            e.setStatusMsg("AI response inserted at cursor")
        } else {
            e.setStatusMsg("No AI response available. Use Ctrl+L to ask AI first")
        }

    case tcell.KeyCtrlA:
        text := tab.buffer.GetText()
        e.clipboard.Copy(text)
        e.setStatusMsg("All text copied to system clipboard")

    case tcell.KeyCtrlC:
        if tab.cursor.Row >= 0 && tab.cursor.Row < tab.buffer.LineCount() {
            line := tab.buffer.GetLine(tab.cursor.Row)
            e.clipboard.Copy(line)
            e.setStatusMsg("Current line copied to system clipboard")
        }

    case tcell.KeyCtrlX:
        if tab.cursor.Row >= 0 && tab.cursor.Row < tab.buffer.LineCount() {
            line := tab.buffer.GetLine(tab.cursor.Row)
            e.clipboard.Copy(line)
            tab.buffer.DeleteLine(tab.cursor.Row)
            if tab.cursor.Row >= tab.buffer.LineCount() && tab.cursor.Row > 0 {
                tab.cursor.Row--
            }
            tab.cursor.Col = 0
            e.ensureCursorValid(tab)
            tab.buffer.SaveState(tab.cursor.Row, tab.cursor.Col)
            e.setStatusMsg("Current line cut to system clipboard")
        }

    case tcell.KeyCtrlV:
        text, err := e.clipboard.Paste()
        if err == nil && text != "" {
            oldRow := tab.cursor.Row
            oldCol := tab.cursor.Col
            tab.buffer.InsertText(tab.cursor.Row, tab.cursor.Col, text)

            lines := strings.Split(text, "\n")
            if len(lines) > 1 {
                tab.cursor.Row = oldRow + len(lines) - 1
                if tab.cursor.Row < 0 {
                    tab.cursor.Row = 0
                }
                if len(lines) > 0 {
                    lastLine := lines[len(lines)-1]
                    tab.cursor.Col = len(lastLine)
                }
            } else {
                tab.cursor.Col = oldCol + len(text)
            }

            e.ensureCursorValid(tab)
            tab.buffer.SaveState(tab.cursor.Row, tab.cursor.Col)
            e.setStatusMsg("System clipboard content pasted")
        } else {
            e.setStatusMsg("Clipboard is empty or unavailable")
        }

    case tcell.KeyCtrlZ:
        if row, col, ok := tab.buffer.Undo(); ok {
            tab.cursor.Row = row
            tab.cursor.Col = col
            e.ensureCursorValid(tab)
            e.setStatusMsg("Undo successful")
        } else {
            e.setStatusMsg("Nothing to undo")
        }

    case tcell.KeyCtrlY:
        if row, col, ok := tab.buffer.Redo(); ok {
            tab.cursor.Row = row
            tab.cursor.Col = col
            e.ensureCursorValid(tab)
            e.setStatusMsg("Redo successful")
        } else {
            e.setStatusMsg("Nothing to redo")
        }

    case tcell.KeyUp:
        if tab.cursor.Row > 0 {
            tab.cursor.Row--
            e.ensureCursorValid(tab)
        }

    case tcell.KeyDown:
        if tab.cursor.Row < tab.buffer.LineCount()-1 {
            tab.cursor.Row++
            e.ensureCursorValid(tab)
        }

    case tcell.KeyLeft:
        if tab.cursor.Col > 0 {
            tab.cursor.Col--
        } else if tab.cursor.Row > 0 {
            tab.cursor.Row--
            tab.cursor.Col = len(tab.buffer.GetLine(tab.cursor.Row))
        }

    case tcell.KeyRight:
        lineLen := len(tab.buffer.GetLine(tab.cursor.Row))
        if tab.cursor.Col < lineLen {
            tab.cursor.Col++
        } else if tab.cursor.Row < tab.buffer.LineCount()-1 {
            tab.cursor.Row++
            tab.cursor.Col = 0
        }

    case tcell.KeyHome:
        if mod&tcell.ModCtrl != 0 {
            tab.cursor.Row = 0
            tab.cursor.Col = 0
            e.ensureCursorValid(tab)
            e.setStatusMsg("Moved to start of file")
        } else {
            tab.cursor.Col = 0
        }

    case tcell.KeyEnd:
        if mod&tcell.ModCtrl != 0 {
            tab.cursor.Row = tab.buffer.LineCount() - 1
            if tab.cursor.Row < 0 {
                tab.cursor.Row = 0
            }
            tab.cursor.Col = len(tab.buffer.GetLine(tab.cursor.Row))
            e.ensureCursorValid(tab)
            e.setStatusMsg("Moved to end of file")
        } else {
            tab.cursor.Col = len(tab.buffer.GetLine(tab.cursor.Row))
        }

    case tcell.KeyPgUp:
        tab.cursor.Row -= e.height
        if tab.cursor.Row < 0 {
            tab.cursor.Row = 0
        }
        e.ensureCursorValid(tab)

    case tcell.KeyPgDn:
        tab.cursor.Row += e.height
        if tab.cursor.Row >= tab.buffer.LineCount() {
            tab.cursor.Row = tab.buffer.LineCount() - 1
        }
        if tab.cursor.Row < 0 {
            tab.cursor.Row = 0
        }
        e.ensureCursorValid(tab)

    case tcell.KeyEnter:
        tab.buffer.InsertNewline(tab.cursor.Row, tab.cursor.Col)
        tab.cursor.Row++
        if tab.cursor.Row < 0 {
            tab.cursor.Row = 0
        }
        tab.cursor.Col = 0
        e.ensureCursorValid(tab)
        tab.buffer.SaveState(tab.cursor.Row, tab.cursor.Col)
        e.quitAttempts = 0

    case tcell.KeyBackspace, tcell.KeyBackspace2:
        if tab.cursor.Col > 0 {
            tab.buffer.DeleteChar(tab.cursor.Row, tab.cursor.Col)
            tab.cursor.Col--
        } else if tab.cursor.Row > 0 {
            prevLineLen := len(tab.buffer.GetLine(tab.cursor.Row - 1))
            tab.buffer.DeleteChar(tab.cursor.Row, tab.cursor.Col)
            tab.cursor.Row--
            tab.cursor.Col = prevLineLen
        }
        e.ensureCursorValid(tab)
        tab.buffer.SaveState(tab.cursor.Row, tab.cursor.Col)
        e.quitAttempts = 0

    case tcell.KeyDelete:
        lineLen := len(tab.buffer.GetLine(tab.cursor.Row))
        if tab.cursor.Col < lineLen {
            tab.buffer.DeleteCharForward(tab.cursor.Row, tab.cursor.Col)
        } else if tab.cursor.Row < tab.buffer.LineCount()-1 {
            nextLine := tab.buffer.GetLine(tab.cursor.Row + 1)
            tab.buffer.AppendToLine(tab.cursor.Row, nextLine)
            tab.buffer.DeleteLine(tab.cursor.Row + 1)
        }
        e.ensureCursorValid(tab)
        tab.buffer.SaveState(tab.cursor.Row, tab.cursor.Col)
        e.quitAttempts = 0

    case tcell.KeyRune:
        if ev.Rune() == '\t' {
            for i := 0; i < 4; i++ {
                tab.buffer.InsertChar(tab.cursor.Row, tab.cursor.Col, ' ')
                tab.cursor.Col++
            }
        } else {
            tab.buffer.InsertChar(tab.cursor.Row, tab.cursor.Col, ev.Rune())
            tab.cursor.Col++
        }
        e.ensureCursorValid(tab)
        tab.buffer.SaveState(tab.cursor.Row, tab.cursor.Col)
        e.quitAttempts = 0
    }

    return true
}


func (e *Editor) handleFindMode(ev *tcell.EventKey) bool {
    switch ev.Key() {
    case tcell.KeyEscape:
        e.mode = ModeNormal
        e.setStatusMsg("Search cancelled")
    case tcell.KeyEnter:
        e.findQuery = e.inputBuffer
        e.findText()
        e.mode = ModeNormal
    case tcell.KeyBackspace, tcell.KeyBackspace2:
        if len(e.inputBuffer) > 0 {
            e.inputBuffer = e.inputBuffer[:len(e.inputBuffer)-1]
        }
        e.setStatusMsg("Find: " + e.inputBuffer)
    case tcell.KeyRune:
        e.inputBuffer += string(ev.Rune())
        e.setStatusMsg("Find: " + e.inputBuffer)
    }
    return true
}

func (e *Editor) handleGotoMode(ev *tcell.EventKey) bool {
    tab := e.tabManager.GetActiveTab()
    if tab == nil || tab.buffer == nil || tab.cursor == nil {
        return true
    }

    switch ev.Key() {
    case tcell.KeyEscape:
        e.mode = ModeNormal
        e.setStatusMsg("Go to line cancelled")
    case tcell.KeyEnter:
        var lineNum int
        _, err := fmt.Sscanf(e.inputBuffer, "%d", &lineNum)
        if err == nil && lineNum > 0 && lineNum <= tab.buffer.LineCount() {
            tab.cursor.Row = lineNum - 1
            tab.cursor.Col = 0
            e.ensureCursorValid(tab)
            e.setStatusMsg(fmt.Sprintf("Jumped to line %d", lineNum))
        } else {
            e.setStatusMsg("Invalid line number")
        }
        e.mode = ModeNormal
    case tcell.KeyBackspace, tcell.KeyBackspace2:
        if len(e.inputBuffer) > 0 {
            e.inputBuffer = e.inputBuffer[:len(e.inputBuffer)-1]
        }
        e.setStatusMsg("Go to line: " + e.inputBuffer)
    case tcell.KeyRune:
        if ev.Rune() >= '0' && ev.Rune() <= '9' {
            e.inputBuffer += string(ev.Rune())
            e.setStatusMsg("Go to line: " + e.inputBuffer)
        }
    }
    return true
}

func (e *Editor) handleLLMMode(ev *tcell.EventKey) bool {
    switch ev.Key() {
    case tcell.KeyEscape:
        e.mode = ModeNormal
        e.setStatusMsg("AI prompt cancelled")
    case tcell.KeyEnter:
        e.llmPrompt = e.inputBuffer
        if e.streamEnabled {
            e.askLLMStream()
        } else {
            e.askLLMAsync()
        }
        e.mode = ModeNormal
    case tcell.KeyBackspace, tcell.KeyBackspace2:
        if len(e.inputBuffer) > 0 {
            e.inputBuffer = e.inputBuffer[:len(e.inputBuffer)-1]
        }
        if e.streamEnabled {
            e.setStatusMsg("Ask AI (streaming): " + e.inputBuffer)
        } else {
            e.setStatusMsg("Ask AI: " + e.inputBuffer)
        }
    case tcell.KeyRune:
        e.inputBuffer += string(ev.Rune())
        if e.streamEnabled {
            e.setStatusMsg("Ask AI (streaming): " + e.inputBuffer)
        } else {
            e.setStatusMsg("Ask AI: " + e.inputBuffer)
        }
    }
    return true
}

func (e *Editor) handleFilenameMode(ev *tcell.EventKey) bool {
    tab := e.tabManager.GetActiveTab()
    if tab == nil || tab.buffer == nil {
        return true
    }

    switch ev.Key() {
    case tcell.KeyEscape:
        e.mode = ModeNormal
        e.setStatusMsg("Save cancelled")
    case tcell.KeyEnter:
        tab.buffer.filename = e.inputBuffer
        e.saveFile()
        e.mode = ModeNormal
    case tcell.KeyBackspace, tcell.KeyBackspace2:
        if len(e.inputBuffer) > 0 {
            e.inputBuffer = e.inputBuffer[:len(e.inputBuffer)-1]
        }
        e.setStatusMsg("Filename: " + e.inputBuffer)
    case tcell.KeyRune:
        e.inputBuffer += string(ev.Rune())
        e.setStatusMsg("Filename: " + e.inputBuffer)
    }
    return true
}

func (e *Editor) saveFile() {
    tab := e.tabManager.GetActiveTab()
    if tab == nil || tab.buffer == nil {
        return
    }

    if tab.buffer.filename == "" {
        e.mode = ModeFilename
        e.inputBuffer = ""
        e.setStatusMsg("Enter filename: ")
        return
    }

    if err := tab.buffer.Save(); err != nil {
        e.setStatusMsg(fmt.Sprintf("Save failed: %v", err))
    } else {
        basename := filepath.Base(tab.buffer.filename)
        e.setStatusMsg(fmt.Sprintf("Saved '%s' (%d lines)", basename, tab.buffer.LineCount()))
    }
}

func (e *Editor) findText() {
    tab := e.tabManager.GetActiveTab()
    if tab == nil || tab.buffer == nil || tab.cursor == nil {
        return
    }

    if e.findQuery == "" {
        e.setStatusMsg("No search query entered")
        return
    }

    totalLines := tab.buffer.LineCount()
    if totalLines == 0 {
        e.setStatusMsg("Buffer is empty")
        return
    }

    startRow := tab.cursor.Row
    startCol := tab.cursor.Col + 1

    if startRow < 0 {
        startRow = 0
    }
    if startRow >= totalLines {
        startRow = totalLines - 1
    }

    for i := 0; i < totalLines; i++ {
        row := (startRow + i) % totalLines
        line := tab.buffer.GetLine(row)

        searchFrom := 0
        if row == startRow && i == 0 {
            searchFrom = startCol
        }

        if searchFrom >= len(line) {
            continue
        }

        lowerLine := strings.ToLower(line[searchFrom:])
        lowerQuery := strings.ToLower(e.findQuery)
        idx := strings.Index(lowerLine, lowerQuery)

        if idx != -1 {
            tab.cursor.Row = row
            tab.cursor.Col = searchFrom + idx
            e.ensureCursorValid(tab)
            e.setStatusMsg(fmt.Sprintf("Found '%s' at line %d, column %d", e.findQuery, row+1, tab.cursor.Col+1))
            return
        }
    }

    e.setStatusMsg(fmt.Sprintf("'%s' not found in document", e.findQuery))
}

func (e *Editor) askLLMAsync() {
    if e.llmPrompt == "" {
        e.setStatusMsg("No prompt entered")
        return
    }

    e.aiMutex.Lock()
    if e.aiInProgress {
        e.aiMutex.Unlock()
        e.setStatusMsg("AI request already in progress. Press Esc to cancel current request")
        return
    }
    e.aiInProgress = true
    e.aiMutex.Unlock()

    if !e.llmClient.IsAvailable() {
        e.aiMutex.Lock()
        e.aiInProgress = false
        e.aiMutex.Unlock()
        e.setStatusMsg("Cannot connect to Ollama. Is it running? Try: ollama serve")
        return
    }

    if err := e.llmClient.CheckModel(); err != nil {
        e.aiMutex.Lock()
        e.aiInProgress = false
        e.aiMutex.Unlock()
        errMsg := err.Error()
        if len(errMsg) > 80 {
            errMsg = errMsg[:77] + "..."
        }
        e.setStatusMsg(fmt.Sprintf("Model error: %s", errMsg))
        return
    }

    e.setStatusMsg("Processing AI request... (Press Esc to cancel)")

    go func() {
        prompt := e.llmPrompt
        
        done := make(chan bool, 1)
        var response string
        var err error
        
        go func() {
            response, err = e.llmClient.GenerateWithCancel(prompt, e.aiCancel)
            done <- true
        }()
        
        select {
        case <-done:
        case <-time.After(90 * time.Second):
            select {
            case e.aiCancel <- true:
            default:
            }
            err = fmt.Errorf("request timeout (90s)")
        }

        e.aiMutex.Lock()
        e.aiInProgress = false
        e.aiMutex.Unlock()

        if err != nil {
            errMsg := err.Error()
            if errMsg == "cancelled" {
                e.setStatusMsg("AI request cancelled by user")
            } else if strings.Contains(errMsg, "cannot connect") {
                e.setStatusMsg("Cannot connect to Ollama. Run: ollama serve")
            } else if strings.Contains(errMsg, "model") && strings.Contains(errMsg, "not found") {
                modelName := e.llmClient.model
                e.setStatusMsg(fmt.Sprintf("Model '%s' not found. Run: ollama pull %s", modelName, modelName))
            } else if strings.Contains(errMsg, "timeout") {
                e.setStatusMsg("AI request timeout. Try a simpler prompt or check Ollama")
            } else {
                if len(errMsg) > 70 {
                    errMsg = errMsg[:67] + "..."
                }
                e.setStatusMsg(fmt.Sprintf("AI error: %s", errMsg))
            }
            e.setLLMResponse("")
            return
        }

        if response == "" {
            e.setStatusMsg("AI returned empty response. Try rephrasing your prompt")
            e.setLLMResponse("")
            return
        }

        e.setLLMResponse(response)
        
        preview := response
        preview = strings.ReplaceAll(preview, "\n", " ")
        preview = strings.ReplaceAll(preview, "\r", "")
        preview = strings.TrimSpace(preview)
        
        if len(preview) > 60 {
            preview = preview[:57] + "..."
        }
        
        if preview == "" {
            preview = "[Response ready]"
        }
        
        responseLines := strings.Count(response, "\n") + 1
        e.setStatusMsg(fmt.Sprintf("AI response ready (%d lines). Preview: %s | Press Ctrl+K to insert", responseLines, preview))
    }()
}

func (e *Editor) askLLMStream() {
    if e.llmPrompt == "" {
        e.setStatusMsg("No prompt entered")
        return
    }

    e.aiMutex.Lock()
    if e.aiInProgress {
        e.aiMutex.Unlock()
        e.setStatusMsg("AI request already in progress. Press Esc to cancel current request")
        return
    }
    e.aiInProgress = true
    e.aiMutex.Unlock()

    if !e.llmClient.IsAvailable() {
        e.aiMutex.Lock()
        e.aiInProgress = false
        e.aiMutex.Unlock()
        e.setStatusMsg("Cannot connect to Ollama. Is it running? Try: ollama serve")
        return
    }

    if err := e.llmClient.CheckModel(); err != nil {
        e.aiMutex.Lock()
        e.aiInProgress = false
        e.aiMutex.Unlock()
        errMsg := err.Error()
        if len(errMsg) > 80 {
            errMsg = errMsg[:77] + "..."
        }
        e.setStatusMsg(fmt.Sprintf("Model error: %s", errMsg))
        return
    }

    e.setLLMResponse("")
    e.setStatusMsg("Streaming AI response... (Press Esc to cancel)")

    go func() {
        prompt := e.llmPrompt
        
        err := e.llmClient.GenerateStream(prompt, e.aiCancel, func(chunk string) {
            e.appendLLMResponse(chunk)
            
            response := e.getLLMResponse()
            preview := response
            preview = strings.ReplaceAll(preview, "\n", " ")
            preview = strings.ReplaceAll(preview, "\r", "")
            preview = strings.TrimSpace(preview)
            
            if len(preview) > 50 {
                preview = preview[:47] + "..."
            }
            
            e.setStatusMsg(fmt.Sprintf("Streaming: %s", preview))
        })

        e.aiMutex.Lock()
        e.aiInProgress = false
        e.aiMutex.Unlock()

        if err != nil {
            errMsg := err.Error()
            if errMsg == "cancelled" {
                e.setStatusMsg("AI stream cancelled by user")
            } else if strings.Contains(errMsg, "cannot connect") {
                e.setStatusMsg("Cannot connect to Ollama. Run: ollama serve")
            } else {
                if len(errMsg) > 70 {
                    errMsg = errMsg[:67] + "..."
                }
                e.setStatusMsg(fmt.Sprintf("Stream error: %s", errMsg))
            }
            response := e.getLLMResponse()
            if response == "" {
                return
            }
        }

        response := e.getLLMResponse()
        if response == "" {
            e.setStatusMsg("AI returned empty response. Try rephrasing your prompt")
            return
        }
        
        responseLines := strings.Count(response, "\n") + 1
        preview := response
        preview = strings.ReplaceAll(preview, "\n", " ")
        preview = strings.ReplaceAll(preview, "\r", "")
        preview = strings.TrimSpace(preview)
        
        if len(preview) > 60 {
            preview = preview[:57] + "..."
        }
        
        e.setStatusMsg(fmt.Sprintf("Stream complete (%d lines). Preview: %s | Press Ctrl+K to insert", responseLines, preview))
    }()
}

func (e *Editor) ensureCursorValid(tab *Tab) {
    if tab == nil || tab.cursor == nil || tab.buffer == nil {
        return
    }

    if tab.cursor.Row < 0 {
        tab.cursor.Row = 0
    }
    maxRow := tab.buffer.LineCount() - 1
    if maxRow < 0 {
        maxRow = 0
    }
    if tab.cursor.Row > maxRow {
        tab.cursor.Row = maxRow
    }

    lineLen := len(tab.buffer.GetLine(tab.cursor.Row))
    if tab.cursor.Col > lineLen {
        tab.cursor.Col = lineLen
    }
    if tab.cursor.Col < 0 {
        tab.cursor.Col = 0
    }
}

func (e *Editor) render() {
    if e.screen == nil {
        return
    }

    tab := e.tabManager.GetActiveTab()
    if tab == nil || tab.buffer == nil || tab.cursor == nil {
        return
    }

    e.screen.Clear()

    e.ensureCursorValid(tab)

    if tab.cursor.Row < tab.offsetRow {
        tab.offsetRow = tab.cursor.Row
    }
    if tab.cursor.Row >= tab.offsetRow+e.height && e.height > 0 {
        tab.offsetRow = tab.cursor.Row - e.height + 1
    }
    if tab.offsetRow < 0 {
        tab.offsetRow = 0
    }

    e.renderTabBar()

    for y := 0; y < e.height; y++ {
        row := y + tab.offsetRow
        screenY := y + 1
        
        if screenY < 0 || screenY >= e.height+3 {
            continue
        }
        
        if row >= tab.buffer.LineCount() {
            e.drawString(0, screenY, "~", tcell.StyleDefault.Foreground(tcell.ColorBlue))
            continue
        }

        line := tab.buffer.GetLine(row)
        e.drawString(0, screenY, line, tcell.StyleDefault)
    }

    e.renderStatusBar()

    screenY := tab.cursor.Row - tab.offsetRow + 1
    screenX := tab.cursor.Col

    if screenX >= e.width {
        screenX = e.width - 1
    }
    if screenX < 0 {
        screenX = 0
    }
    if screenY >= e.height+1 {
        screenY = e.height
    }
    if screenY < 1 {
        screenY = 1
    }

    e.screen.ShowCursor(screenX, screenY)
    e.screen.Show()
}

func (e *Editor) renderTabBar() {
    y := 0
    
    style := tcell.StyleDefault.
        Background(tcell.ColorDarkBlue).
        Foreground(tcell.ColorWhite)
    
    activeStyle := tcell.StyleDefault.
        Background(tcell.ColorBlue).
        Foreground(tcell.ColorWhite).
        Bold(true)

    for x := 0; x < e.width; x++ {
        if y >= 0 && y < e.height+3 {
            e.screen.SetContent(x, y, ' ', nil, style)
        }
    }

    x := 0
    tabCount := e.tabManager.GetTabCount()
    
    for i := 0; i < tabCount && x < e.width; i++ {
        tabName := e.tabManager.GetTabName(i)
        isActive := e.tabManager.IsActiveTab(i)
        
        tabLabel := fmt.Sprintf(" %d:%s ", i+1, tabName)
        
        if len(tabLabel) > 20 {
            tabLabel = tabLabel[:17] + ".. "
        }
        
        tabStyle := style
        if isActive {
            tabStyle = activeStyle
        }
        
        if x+len(tabLabel) > e.width {
            break
        }
        
        for _, r := range tabLabel {
            if x >= e.width {
                break
            }
            if y >= 0 && y < e.height+3 {
                e.screen.SetContent(x, y, r, nil, tabStyle)
            }
            x++
        }
        
        if i < tabCount-1 && x < e.width {
            if y >= 0 && y < e.height+3 {
                e.screen.SetContent(x, y, '│', nil, style)
            }
            x++
        }
    }
}

func (e *Editor) renderStatusBar() {
    tab := e.tabManager.GetActiveTab()
    if tab == nil || tab.buffer == nil || tab.cursor == nil {
        return
    }

    y := e.height + 1
    if y < 0 || y >= e.height+3 {
        return
    }

    style := tcell.StyleDefault.
        Background(tcell.ColorWhite).
        Foreground(tcell.ColorBlack)

    e.aiMutex.Lock()
    inProgress := e.aiInProgress
    e.aiMutex.Unlock()

    if inProgress {
        style = tcell.StyleDefault.
            Background(tcell.ColorYellow).
            Foreground(tcell.ColorBlack)
    }

    for x := 0; x < e.width; x++ {
        if y >= 0 && y < e.height+3 {
            e.screen.SetContent(x, y, ' ', nil, style)
        }
        if y+1 >= 0 && y+1 < e.height+3 {
            e.screen.SetContent(x, y+1, ' ', nil, style)
        }
    }

    statusMsg := e.getStatusMsg()
    if len(statusMsg) > e.width && e.width > 3 {
        statusMsg = statusMsg[:e.width-3] + "..."
    }
    e.drawString(0, y, statusMsg, style)

    modMark := ""
    if tab.buffer.modified {
        modMark = " [+]"
    }
    filename := tab.buffer.filename
    if filename == "" {
        filename = "[No Name]"
    } else {
        filename = filepath.Base(filename)
        if len(filename) > 20 {
            filename = filename[:17] + "..."
        }
    }

    info := fmt.Sprintf("%s%s | Ln %d/%d | Col %d | Tab %d/%d",
        filename, modMark, tab.cursor.Row+1, tab.buffer.LineCount(), tab.cursor.Col+1,
        e.tabManager.activeTab+1, e.tabManager.GetTabCount())

    if len(info) > e.width && e.width > 0 {
        info = info[:e.width]
    }

    if y+1 >= 0 && y+1 < e.height+3 {
        e.drawString(0, y+1, info, style)
    }
}

func (e *Editor) drawString(x, y int, s string, style tcell.Style) {
    if e.screen == nil || y < 0 || y >= e.height+3 || x < 0 {
        return
    }

    for i, r := range s {
        posX := x + i
        if posX >= e.width || posX < 0 {
            break
        }
        e.screen.SetContent(posX, y, r, nil, style)
    }
}

func main() {
    ollamaURL := flag.String("ollama", "http://localhost:11434", "Ollama API URL")
    model := flag.String("model", "llama2", "LLM model to use")
    streamEnabled := flag.Bool("stream", false, "Enable streaming AI responses")
    showVersion := flag.Bool("version", false, "Show version")
    showHelp := flag.Bool("help", false, "Show help")

    flag.Parse()

    if *showVersion {
        fmt.Printf("GoEdit v%s\n", version)
        fmt.Println("Copyright © Prof. Dr. Michael Stal, 2025")
        os.Exit(0)
    }

    if *showHelp {
        printHelp()
        os.Exit(0)
    }

    var filenames []string
    for i := 0; i < flag.NArg(); i++ {
        filename := flag.Arg(i)
        absPath, err := filepath.Abs(filename)
        if err == nil {
            filename = absPath
        }
        filenames = append(filenames, filename)
    }

    ed, err := NewEditor(filenames, *ollamaURL, *model, *streamEnabled)
    if err != nil {
        log.Fatalf("Failed to create editor: %v", err)
    }

    if err := ed.Run(); err != nil {
        log.Fatalf("Editor error: %v", err)
    }
}

func printHelp() {
    fmt.Println("GoEdit v2.0 - A powerful terminal text editor with AI assistance")
    fmt.Println("Copyright © Prof. Dr. Michael Stal, 2025")
    fmt.Println("\nUsage:")
    fmt.Println("  goedit [options] [file1] [file2] ...")
    fmt.Println("\nOptions:")
    fmt.Println("  -ollama string    Ollama API URL (default: http://localhost:11434)")
    fmt.Println("  -model string     LLM model to use (default: llama2)")
    fmt.Println("  -stream           Enable streaming AI responses")
    fmt.Println("  -version          Show version")
    fmt.Println("  -help             Show this help")
    fmt.Println("\nKeyboard Shortcuts:")
    fmt.Println("  File Operations:")
    fmt.Println("    Ctrl+S         Save current file")
    fmt.Println("    Ctrl+Q         Quit editor")
    fmt.Println("    Ctrl+T         New tab")
    fmt.Println("    Ctrl+W         Close current tab")
    fmt.Println("    Tab            Next tab")
    fmt.Println("    Shift+Tab      Previous tab")
    fmt.Println("\n  Editing:")
    fmt.Println("    Ctrl+A         Copy all text to system clipboard")
    fmt.Println("    Ctrl+C         Copy current line to system clipboard")
    fmt.Println("    Ctrl+X         Cut current line to system clipboard")
    fmt.Println("    Ctrl+V         Paste from system clipboard")
    fmt.Println("    Ctrl+Z         Undo")
    fmt.Println("    Ctrl+Y         Redo")
    fmt.Println("\n  Navigation:")
    fmt.Println("    Ctrl+F         Find text")
    fmt.Println("    Ctrl+G         Go to line")
    fmt.Println("    Home/End       Line start/end")
    fmt.Println("    Ctrl+Home/End  File start/end")
    fmt.Println("    Page Up/Down   Scroll page")
    fmt.Println("\n  AI Assistant:")
    fmt.Println("    Ctrl+L         Ask AI (with optional streaming)")
    fmt.Println("    Ctrl+K         Insert AI response at cursor")
    fmt.Println("    Esc            Cancel AI request")
    fmt.Println("\nExamples:")
    fmt.Println("  goedit file.txt")
    fmt.Println("  goedit file1.go file2.go file3.go")
    fmt.Println("  goedit -stream -model codellama main.go")
}			
