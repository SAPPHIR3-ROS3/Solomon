package config

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/modelsapi"
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

func PickModelInteractive(stdin io.Reader, p *Provider, providerLabel string) (string, error) {
	ids, err := modelsapi.List(p.BaseURL, p.APIKey)
	if err != nil {
		return "", fmt.Errorf("list models: %w", err)
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("no models returned by API")
	}
	br := bufio.NewScanner(stdin)
	if len(ids) <= 20 {
		for i, id := range ids {
			fmt.Printf("%d\t%s[%s]\n", i, id, providerLabel)
		}
	} else {
		for i := 0; i < 20; i++ {
			fmt.Printf("%d\t%s[%s]\n", i, ids[i], providerLabel)
		}
		fmt.Println("...")
	}
	for {
		if len(ids) <= 20 {
			fmt.Printf("Select model number (0-%d) or paste exact model id: ", len(ids)-1)
		} else {
			fmt.Print("Enter index 0–19, 20 to type a model id, or paste exact model id: ")
		}
		br.Scan()
		line := strings.TrimSpace(br.Text())
		if line == "" {
			fmt.Println("Invalid: empty input.")
			continue
		}
		if len(ids) > 20 {
			if AllDigits(line) {
				n, err := strconv.Atoi(line)
				if err != nil {
					fmt.Println("Invalid: not a valid number.")
					continue
				}
				if n >= 0 && n < 20 {
					return ids[n], nil
				}
				if n == 20 {
					for {
						fmt.Print("Model id: ")
						br.Scan()
						s := strings.TrimSpace(br.Text())
						if s == "" {
							fmt.Println("Invalid: empty model id.")
							continue
						}
						if idInSlice(ids, s) {
							return s, nil
						}
						fmt.Printf("Invalid: model id %q is not in the API model list.\n", s)
					}
				}
				fmt.Println("Invalid: index must be 0–19 or 20 to enter an id.")
				continue
			}
			if idInSlice(ids, line) {
				return line, nil
			}
			fmt.Printf("Invalid: model id %q not found in the model list.\n", line)
			continue
		}

		if AllDigits(line) {
			n, err := strconv.Atoi(line)
			if err != nil {
				fmt.Println("Invalid: not a valid number.")
				continue
			}
			if n >= 0 && n < len(ids) {
				return ids[n], nil
			}
			fmt.Printf("Invalid: index must be between 0 and %d.\n", len(ids)-1)
			continue
		}
		if idInSlice(ids, line) {
			return line, nil
		}
		fmt.Printf("Invalid: model id %q not found in the model list.\n", line)
	}
}
