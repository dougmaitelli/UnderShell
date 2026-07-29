package admin

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

func (h *Handler) RunConsole(ctx context.Context, input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		result, err := h.ExecuteConsole(ctx, line)
		if err != nil {
			_, _ = fmt.Fprintf(output, "command error: %v\n", err)
			continue
		}
		_, _ = fmt.Fprintln(output, result)
	}
	return scanner.Err()
}
