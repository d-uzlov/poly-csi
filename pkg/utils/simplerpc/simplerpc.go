package simplerpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func MakeHandler[In any, Out any](handle func(ctx context.Context, input In) Out) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, fmt.Sprintf("expected POST method but got '%s'", r.Method), http.StatusMethodNotAllowed)
			return
		}

		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, fmt.Sprintf("expected Content-Type=application/json but got '%s'", r.Header.Get("Content-Type")), http.StatusBadRequest)
			return
		}

		limitedReader := http.MaxBytesReader(w, r.Body, 1<<20)

		var in In
		err := json.NewDecoder(limitedReader).Decode(&in)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to parse request body as protojson: %s", err.Error()), http.StatusBadRequest)
			return
		}

		out := handle(r.Context(), in)

		encoded, err := json.Marshal(out)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal result: %s", err.Error()), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, writeErr := w.Write(encoded)
		if writeErr != nil {
			// logger.Error("failed to write result", "error", writeErr)
			return
		}
	}
}

func MakeQuery[In any, Out any](ctx context.Context, client http.Client, url string, in In, out *Out) error {
	b, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("create request object: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("remote error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("remote response %s: %s", resp.Status, string(body))
	}

	err = json.Unmarshal(body, out)
	if err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	return nil
}
