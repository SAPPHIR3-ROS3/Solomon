package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func AllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func idInSlice(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

type ModelPickChoice struct {
	ProviderName string
	ModelID      string
	Changed      bool
}

func PickModelInteractive(stdin io.Reader, out io.Writer, p *Provider, providerLabel string, ids []string, allowSkip bool) (string, error) {
	if stdin == nil {
		stdin = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("no models returned by API")
	}
	br := bufio.NewScanner(stdin)
	if len(ids) <= 20 {
		for i, id := range ids {
			fmt.Fprintf(out, "%d\t%s[%s]\n", i, id, providerLabel)
		}
	} else {
		for i := 0; i < 20; i++ {
			fmt.Fprintf(out, "%d\t%s[%s]\n", i, ids[i], providerLabel)
		}
		fmt.Fprintln(out, "...")
	}
	for {
		if len(ids) <= 20 {
			if allowSkip {
				fmt.Fprintf(out, "Select model number (0-%d), paste exact model id, or skip for default [%s]: ", len(ids)-1, ids[0])
			} else {
				fmt.Fprintf(out, "Select model number (0-%d) or paste exact model id: ", len(ids)-1)
			}
		} else if allowSkip {
			fmt.Fprintf(out, "Enter index 0–19, 20 to type a model id, paste exact model id, or skip for default [%s]: ", ids[0])
		} else {
			fmt.Fprint(out, "Enter index 0–19, 20 to type a model id, or paste exact model id: ")
		}
		if !br.Scan() {
			if err := br.Err(); err != nil {
				return "", err
			}
			return "", fmt.Errorf("unexpected end of input")
		}
		line := strings.TrimSpace(br.Text())
		if allowSkip && isSkipInput(line) {
			PrintConfigSkipHint(out, "current_model")
			return ids[0], nil
		}
		if line == "" {
			if allowSkip {
				fmt.Fprintln(out, "Required: choose a model, paste an id, or type skip.")
			} else {
				fmt.Fprintln(out, "Required: choose a model or paste an id.")
			}
			continue
		}
		if len(ids) > 20 {
			if AllDigits(line) {
				n, err := strconv.Atoi(line)
				if err != nil {
					fmt.Fprintln(out, "Invalid: not a valid number.")
					continue
				}
				if n >= 0 && n < 20 {
					return ids[n], nil
				}
				if n == 20 {
					for {
						fmt.Fprint(out, "Model id: ")
						if !br.Scan() {
							if err := br.Err(); err != nil {
								return "", err
							}
							return "", fmt.Errorf("unexpected end of input")
						}
						s := strings.TrimSpace(br.Text())
						if allowSkip && isSkipInput(s) {
							PrintConfigSkipHint(out, "current_model")
							return ids[0], nil
						}
						if s == "" {
							if allowSkip {
								fmt.Fprintln(out, "Required: enter a model id or type skip.")
							} else {
								fmt.Fprintln(out, "Required: enter a model id.")
							}
							continue
						}
						if idInSlice(ids, s) {
							return s, nil
						}
						fmt.Fprintf(out, "Invalid: model id %q is not in the API model list.\n", s)
					}
				}
				fmt.Fprintln(out, "Invalid: index must be 0–19 or 20 to enter an id.")
				continue
			}
			if idInSlice(ids, line) {
				return line, nil
			}
			fmt.Fprintf(out, "Invalid: model id %q not found in the model list.\n", line)
			continue
		}
		if AllDigits(line) {
			n, err := strconv.Atoi(line)
			if err != nil {
				fmt.Fprintln(out, "Invalid: not a valid number.")
				continue
			}
			if n >= 0 && n < len(ids) {
				return ids[n], nil
			}
			fmt.Fprintf(out, "Invalid: index must be between 0 and %d.\n", len(ids)-1)
			continue
		}
		if idInSlice(ids, line) {
			return line, nil
		}
		fmt.Fprintf(out, "Invalid: model id %q not found in the model list.\n", line)
	}
}

func PickModelAfterAdd(stdin io.Reader, out io.Writer, prevProv, prevModel, newProvName string, newIDs []string, allowSkip bool) (ModelPickChoice, error) {
	if stdin == nil {
		stdin = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	if len(newIDs) == 0 {
		return ModelPickChoice{}, fmt.Errorf("no models returned by API")
	}
	br := bufio.NewScanner(stdin)
	const maxShown = 20
	nShownNew := len(newIDs)
	truncated := false
	if nShownNew > maxShown {
		nShownNew = maxShown
		truncated = true
	}
	fmt.Fprintf(out, "0\t%s\t[%s]\t(current)\n", prevModel, prevProv)
	for i := 0; i < nShownNew; i++ {
		fmt.Fprintf(out, "%d\t%s\t[%s]\n", i+1, newIDs[i], newProvName)
	}
	if truncated {
		fmt.Fprintln(out, "...")
	}
	pasteIdx := 21
	for {
		fmt.Fprintf(out, "Select: 0 = keep current provider/model")
		if nShownNew > 0 {
			fmt.Fprintf(out, ", 1-%d = model on %s", nShownNew, newProvName)
		}
		if truncated {
			fmt.Fprintf(out, ", %d = enter model id", pasteIdx)
		}
		fmt.Fprint(out, ", paste exact model id for the new provider")
		if allowSkip {
			fmt.Fprint(out, ", or skip to keep current")
		}
		fmt.Fprint(out, "\n> ")
		if !br.Scan() {
			if err := br.Err(); err != nil {
				return ModelPickChoice{}, err
			}
			return ModelPickChoice{}, fmt.Errorf("unexpected end of input")
		}
		line := strings.TrimSpace(br.Text())
		if allowSkip && isSkipInput(line) {
			PrintConfigSkipHint(out, "current_model")
			return ModelPickChoice{ProviderName: prevProv, ModelID: prevModel, Changed: false}, nil
		}
		if line == "" {
			if allowSkip {
				fmt.Fprintln(out, "Required: choose an option or type skip.")
			} else {
				fmt.Fprintln(out, "Required: choose an option.")
			}
			continue
		}
		if line == "0" {
			return ModelPickChoice{ProviderName: prevProv, ModelID: prevModel, Changed: false}, nil
		}
		if nShownNew > 0 && AllDigits(line) {
			n, err := strconv.Atoi(line)
			if err == nil && n >= 1 && n <= nShownNew {
				return ModelPickChoice{ProviderName: newProvName, ModelID: newIDs[n-1], Changed: true}, nil
			}
			if truncated && n == pasteIdx {
				for {
					fmt.Fprint(out, "Model id: ")
					if !br.Scan() {
						if err := br.Err(); err != nil {
							return ModelPickChoice{}, err
						}
						return ModelPickChoice{}, fmt.Errorf("unexpected end of input")
					}
					id := strings.TrimSpace(br.Text())
					if allowSkip && isSkipInput(id) {
						PrintConfigSkipHint(out, "current_model")
						return ModelPickChoice{ProviderName: prevProv, ModelID: prevModel, Changed: false}, nil
					}
					if id == "" {
						if allowSkip {
							fmt.Fprintln(out, "Required: enter a model id or type skip.")
						} else {
							fmt.Fprintln(out, "Required: enter a model id.")
						}
						continue
					}
					if err := resolvePasteNewProvider(newProvName, newIDs, id); err != nil {
						fmt.Fprintf(out, "Invalid: %v\n", err)
						continue
					}
					return ModelPickChoice{ProviderName: newProvName, ModelID: id, Changed: true}, nil
				}
			}
		}
		if err := resolvePasteNewProvider(newProvName, newIDs, line); err != nil {
			fmt.Fprintf(out, "Invalid: %v\n", err)
			continue
		}
		return ModelPickChoice{ProviderName: newProvName, ModelID: line, Changed: true}, nil
	}
}

func resolvePasteNewProvider(newProvName string, newIDs []string, id string) error {
	if len(newIDs) == 0 {
		return nil
	}
	for _, mid := range newIDs {
		if mid == id {
			return nil
		}
	}
	return fmt.Errorf("model id %q not in models returned by this provider", id)
}
