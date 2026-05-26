package config

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

const MaxModelPickerEntries = 25

func readModelPickLine(pio PromptIO, prompt string) (string, error) {
	line, err := ReadPromptLine(pio, prompt)
	if err != nil {
		if err == io.EOF {
			return "", fmt.Errorf("unexpected end of input")
		}
		return "", err
	}
	return line, nil
}

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

func PickModelInteractive(pio PromptIO, p *Provider, providerLabel string, ids []string, allowSkip bool) (string, error) {
	out := pio.promptOut()
	if len(ids) == 0 {
		return "", fmt.Errorf("no models returned by API")
	}
	cursorPick := p != nil && p.IsCursorAPI()
	maxList := MaxModelPickerEntries
	lastIdx := maxList - 1
	pasteEntryIdx := maxList
	if len(ids) <= maxList {
		for i, id := range ids {
			fmt.Fprintf(out, "%d\t%s[%s]\n", i, id, providerLabel)
		}
	} else {
		for i := 0; i < maxList; i++ {
			fmt.Fprintf(out, "%d\t%s[%s]\n", i, ids[i], providerLabel)
		}
		fmt.Fprintln(out, "...")
	}
	for {
		var prompt string
		if len(ids) <= maxList {
			if allowSkip && cursorPick {
				prompt = fmt.Sprintf("Select model number (0-%d), paste exact model id, or skip to use %s: ", len(ids)-1, CursorAPIDefaultModelID)
			} else if allowSkip {
				prompt = fmt.Sprintf("Select model number (0-%d), paste exact model id, or skip for default [%s]: ", len(ids)-1, ids[0])
			} else {
				prompt = fmt.Sprintf("Select model number (0-%d) or paste exact model id: ", len(ids)-1)
			}
		} else if allowSkip && cursorPick {
			prompt = fmt.Sprintf("Enter index 0–%d, %d to type a model id, paste exact model id, or skip to use %s: ", lastIdx, pasteEntryIdx, CursorAPIDefaultModelID)
		} else if allowSkip {
			prompt = fmt.Sprintf("Enter index 0–%d, %d to type a model id, paste exact model id, or skip for default [%s]: ", lastIdx, pasteEntryIdx, ids[0])
		} else {
			prompt = fmt.Sprintf("Enter index 0–%d, %d to type a model id, or paste exact model id: ", lastIdx, pasteEntryIdx)
		}
		line, err := readModelPickLine(pio, prompt)
		if err != nil {
			return "", err
		}
		if allowSkip && isSkipInput(line) {
			PrintConfigSkipHint(out, "current_model")
			if cursorPick {
				return CursorAPIDefaultModelID, nil
			}
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
		if len(ids) > maxList {
			if AllDigits(line) {
				n, err := strconv.Atoi(line)
				if err != nil {
					fmt.Fprintln(out, "Invalid: not a valid number.")
					continue
				}
				if n >= 0 && n < maxList {
					return ids[n], nil
				}
				if n == pasteEntryIdx {
					for {
						s, err := readModelPickLine(pio, "Model id: ")
						if err != nil {
							return "", err
						}
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
				fmt.Fprintf(out, "Invalid: index must be 0–%d or %d to enter an id.\n", lastIdx, pasteEntryIdx)
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

func PickModelAfterAdd(pio PromptIO, prevProv, prevModel, newProvName string, newIDs []string, allowSkip bool) (ModelPickChoice, error) {
	out := pio.promptOut()
	if len(newIDs) == 0 {
		return ModelPickChoice{}, fmt.Errorf("no models returned by API")
	}
	maxShown := MaxModelPickerEntries
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
	pasteIdx := maxShown + 1
	printPickAfterAddHelp(out, nShownNew, newProvName, truncated, pasteIdx, allowSkip)
	for {
		line, err := readModelPickLine(pio, pickAfterAddReadPrompt())
		if err != nil {
			return ModelPickChoice{}, err
		}
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
					id, err := readModelPickLine(pio, "Model id: ")
					if err != nil {
						return ModelPickChoice{}, err
					}
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

func printPickAfterAddHelp(out io.Writer, nShownNew int, newProvName string, truncated bool, pasteIdx int, allowSkip bool) {
	var b strings.Builder
	b.WriteString("Select: 0 = keep current provider/model")
	if nShownNew > 0 {
		fmt.Fprintf(&b, ", 1-%d = model on %s", nShownNew, newProvName)
	}
	if truncated {
		fmt.Fprintf(&b, ", %d = enter model id", pasteIdx)
	}
	b.WriteString(", paste exact model id for the new provider")
	if allowSkip {
		b.WriteString(", or skip to keep current")
	}
	fmt.Fprintln(out, b.String())
}

func pickAfterAddReadPrompt() string {
	return "> "
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
