// super/middleware/panic_recovery.go
package middleware

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strings"

	constants "github.com/nicklasjeppesen/going_internal/super/constants"
	"github.com/nicklasjeppesen/going_internal/super/util"
)

type StackFrame struct {
	File     string
	Line     int
	Function string
	Source   []SourceLine // Lines around the error
	IsApp    bool         // Highlight if it's your own code
	ErrorAt  int          // Which line is the actual error
}

type SourceLine struct {
	Number    int
	Content   string
	IsErrorAt bool
}

type ErrorPageData struct {
	ErrorMessage string
	ErrorType    string
	Frames       []StackFrame
	RequestID    string
	Method       string
	URL          string
	GoVersion    string
}

func PanicRecovery(next http.Handler) http.Handler {
	tmpl, err := template.New("error").ParseFiles("internal/resources/templates/errors/500.html")
	if err != nil {
		log.Printf("WARNING: Could not load 500 template: %v", err)
		tmpl = nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				// Collect raw stack
				rawStack := debug.Stack()
				log.Printf("PANIC RECOVERED: %v\nStack Trace:\n%s", rec, rawStack)

				frames := parseStackTrace()
				errorData := ErrorPageData{
					ErrorMessage: fmt.Sprintf("%v", rec),
					ErrorType:    fmt.Sprintf("%T", rec),
					Frames:       frames,
					Method:       r.Method,
					URL:          r.URL.String(),
					GoVersion:    runtime.Version(),
				}

				debugMode := util.GetEnv(constants.APP_Debug, "") == "true"
				w.WriteHeader(http.StatusInternalServerError)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")

				if tmpl != nil && debugMode {
					if renderErr := tmpl.ExecuteTemplate(w, "500.html", errorData); renderErr != nil {
						log.Printf("Failed to render error template: %v", renderErr)
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					}
					return
				}

				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// parseStackTrace walks the real runtime call stack and extracts file/line/func info
func parseStackTrace() []StackFrame {
	var frames []StackFrame

	// Walk up to 20 frames, skip runtime internals
	pcs := make([]uintptr, 20)
	// Skip: parseStackTrace + PanicRecovery defer + runtime.gopanic = 4
	n := runtime.Callers(4, pcs)
	callFrames := runtime.CallersFrames(pcs[:n])

	for {
		frame, more := callFrames.Next()

		// Skip Go runtime internals
		if strings.HasPrefix(frame.Function, "runtime.") {
			if !more {
				break
			}
			continue
		}

		sf := StackFrame{
			File:     frame.File,
			Line:     frame.Line,
			Function: frame.Function,
			IsApp:    isAppCode(frame.File),
		}

		sf.Source = readSourceLines(frame.File, frame.Line, 5)
		sf.ErrorAt = frame.Line
		frames = append(frames, sf)

		if !more {
			break
		}
	}

	return frames
}

// isAppCode returns true if the file belongs to your project (not stdlib/vendor)
func isAppCode(file string) bool {
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	return strings.HasPrefix(file, cwd)
}

// readSourceLines reads lines around the error line from the source file
func readSourceLines(file string, errorLine int, context int) []SourceLine {
	content, err := os.ReadFile(file)
	if err != nil {
		return nil
	}

	allLines := strings.Split(string(content), "\n")
	start := max(0, errorLine-context-1)
	end := min(len(allLines), errorLine+context)

	var lines []SourceLine
	for i := start; i < end; i++ {
		lines = append(lines, SourceLine{
			Number:    i + 1,
			Content:   allLines[i],
			IsErrorAt: (i + 1) == errorLine,
		})
	}
	return lines
}
