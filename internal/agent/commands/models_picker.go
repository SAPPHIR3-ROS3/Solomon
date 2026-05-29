package commands

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands/connect"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

const slashModelMoreCmd = ">"

type slashPickerDisplayRow struct {
	index int
	pr    pickerRow
}

type slashModelPickerCtx struct {
	d                   Deps
	fullCatalog         []ListedModel
	providerCatalog     map[string][]ListedModel
	filterProv          string
	displayedKeys       map[string]bool
	stickyRecents       []ListedModel
	firstPageRecents    int
	page                int
	indexTable          map[int]pickerRow
	nextIndex           int
	lastPageCatalogFrom int
}

func (c *slashModelPickerCtx) cur() ListedModel {
	return ListedModel{Prov: c.d.Provider().Name, Model: c.d.Model()}
}

func (c *slashModelPickerCtx) filteredCatalog() []ListedModel {
	if c.filterProv == "" {
		return c.fullCatalog
	}
	if cat, ok := c.providerCatalog[c.filterProv]; ok && len(cat) > 0 {
		return cat
	}
	var out []ListedModel
	for i := range c.fullCatalog {
		if c.fullCatalog[i].Prov == c.filterProv {
			out = append(out, c.fullCatalog[i])
		}
	}
	return out
}

func (c *slashModelPickerCtx) providerFilterActive() bool {
	return strings.TrimSpace(c.filterProv) != ""
}

func (c *slashModelPickerCtx) ensureProviderCatalog(ctx context.Context, cfg *config.Root, prov string) error {
	if c.providerCatalog == nil {
		c.providerCatalog = map[string][]ListedModel{}
	}
	if _, ok := c.providerCatalog[prov]; ok {
		return nil
	}
	if cat, ok := cachedProviderCatalogSlice(cfg, prov); ok {
		c.providerCatalog[prov] = cat
		return nil
	}
	p := config.ProviderByName(cfg, prov)
	if p == nil {
		return fmt.Errorf("provider %q not found", prov)
	}
	pp := *p
	ids, err := connect.ListModelsForProviderAll(ctx, cfg, &pp)
	if err != nil {
		return err
	}
	c.providerCatalog[prov] = orderListedModelsByProvider(orderListedModelsFromIDs(prov, ids), prov)
	return nil
}

func (c *slashModelPickerCtx) resetView() {
	c.page = 0
	c.firstPageRecents = 0
	c.displayedKeys = map[string]bool{}
	c.indexTable = map[int]pickerRow{}
	c.nextIndex = 0
	c.lastPageCatalogFrom = 0
	c.stickyRecents = nil
}

func (c *slashModelPickerCtx) resolveProviderFilter(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", false
	}
	for _, p := range config.ProviderList(c.d.Cfg) {
		if strings.EqualFold(strings.TrimSpace(p.Name), line) {
			return p.Name, true
		}
	}
	return "", false
}

func (c *slashModelPickerCtx) markDisplayed(rows []pickerRow) {
	for i := range rows {
		c.displayedKeys[lmKey(rows[i].lm)] = true
	}
}

func (c *slashModelPickerCtx) hasUndisplayed() bool {
	for _, lm := range c.filteredCatalog() {
		if !c.displayedKeys[lmKey(lm)] {
			return true
		}
	}
	return false
}

func (c *slashModelPickerCtx) catalogPageSize() int {
	n := config.MaxModelPickerEntries - c.firstPageRecents
	if n < 1 {
		return 1
	}
	return n
}

func (c *slashModelPickerCtx) registerRow(pr pickerRow) {
	c.indexTable[c.nextIndex] = pr
	c.nextIndex++
}

func (c *slashModelPickerCtx) registerRows(rows []pickerRow) {
	for i := range rows {
		c.registerRow(rows[i])
	}
}

func (c *slashModelPickerCtx) maxIndex() int {
	if c.nextIndex == 0 {
		return 0
	}
	return c.nextIndex - 1
}

func (c *slashModelPickerCtx) buildDisplay() ([]slashPickerDisplayRow, bool) {
	if c.page == 0 {
		return c.buildFirstPage()
	}
	return c.buildNextPage()
}

func (c *slashModelPickerCtx) buildFirstPage() ([]slashPickerDisplayRow, bool) {
	if c.providerFilterActive() {
		return c.buildProviderOnlyFirstPage()
	}
	catalog := c.filteredCatalog()
	cur := c.cur()
	claimed := map[string]bool{lmKey(cur): true}

	recents := pickRecentListed(c.d, catalog, cur, claimed, 5)

	var chatgptSlots []ListedModel
	{
		chatgptSlots = pickChatGPTSubFamilySlots(catalog, cur, claimed, 5)
		for i := range chatgptSlots {
			claimed[lmKey(chatgptSlots[i])] = true
		}
		claimOtherChatGPTSubCatalog(catalog, cur, claimed)
	}
	claudeSub := pickClaudeSubListed(c.d, catalog, cur, claimed, 5)
	c.stickyRecents = recents
	c.firstPageRecents = len(recents)

	need := config.MaxModelPickerEntries - len(recents) - len(chatgptSlots) - len(claudeSub)
	if need < 0 {
		need = 0
	}
	rest := fillProviderPicksFiltered(catalog, claimed, need, "", true)
	catalogRows := append([]ListedModel(nil), chatgptSlots...)
	catalogRows = append(catalogRows, rest...)

	rows := assemblePickerRows(cur, recents, claudeSub, catalogRows)
	c.indexTable = map[int]pickerRow{}
	c.nextIndex = 0
	c.registerRows(rows)
	c.markDisplayed(rows)

	var disp []slashPickerDisplayRow
	for idx := 0; idx < c.nextIndex; idx++ {
		disp = append(disp, slashPickerDisplayRow{index: idx, pr: c.indexTable[idx]})
	}
	return disp, c.hasUndisplayed()
}

func (c *slashModelPickerCtx) buildProviderOnlyFirstPage() ([]slashPickerDisplayRow, bool) {
	catalog := c.filteredCatalog()
	cur := c.cur()
	claimed := map[string]bool{lmKey(cur): true}
	prov := c.filterProv

	recents := pickRecentListedForProvider(c.d, catalog, cur, claimed, 5, prov)
	c.stickyRecents = recents
	c.firstPageRecents = len(recents)

	need := config.MaxModelPickerEntries - len(recents)
	if need < 0 {
		need = 0
	}
	batch := fillProviderCatalogBatch(catalog, claimed, need, prov)

	rows := assemblePickerRows(cur, recents, nil, batch)
	c.indexTable = map[int]pickerRow{}
	c.nextIndex = 0
	c.registerRows(rows)
	c.markDisplayed(rows)

	var disp []slashPickerDisplayRow
	for idx := 0; idx < c.nextIndex; idx++ {
		disp = append(disp, slashPickerDisplayRow{index: idx, pr: c.indexTable[idx]})
	}
	return disp, c.hasUndisplayed()
}

func (c *slashModelPickerCtx) buildNextPage() ([]slashPickerDisplayRow, bool) {
	catalog := c.filteredCatalog()
	need := c.catalogPageSize()
	c.lastPageCatalogFrom = c.nextIndex
	var batch []ListedModel
	if c.providerFilterActive() {
		batch = fillProviderCatalogBatch(catalog, c.displayedKeys, need, c.filterProv)
	} else {
		batch = fillProviderPicksFiltered(catalog, c.displayedKeys, need, "", false)
	}

	var batchRows []pickerRow
	for i := range batch {
		batchRows = append(batchRows, pickerRow{lm: batch[i], section: sectionCatalog})
	}
	c.registerRows(batchRows)
	c.markDisplayed(batchRows)

	disp := c.stickyDisplayRows()
	for idx := c.lastPageCatalogFrom; idx < c.nextIndex; idx++ {
		disp = append(disp, slashPickerDisplayRow{index: idx, pr: c.indexTable[idx]})
	}
	return disp, c.hasUndisplayed()
}

func (c *slashModelPickerCtx) stickyDisplayRows() []slashPickerDisplayRow {
	var disp []slashPickerDisplayRow
	if pr, ok := c.indexTable[0]; ok {
		disp = append(disp, slashPickerDisplayRow{index: 0, pr: pr})
	}
	for i := range c.stickyRecents {
		idx := i + 1
		pr, ok := c.indexTable[idx]
		if !ok {
			pr = pickerRow{lm: c.stickyRecents[i], section: sectionRecent}
		}
		disp = append(disp, slashPickerDisplayRow{index: idx, pr: pr})
	}
	return disp
}

func fillProviderPicksFiltered(catalog []ListedModel, skip map[string]bool, need int, onlyProv string, firstPage bool) []ListedModel {
	if need <= 0 {
		return nil
	}
	var out []ListedModel
	for i := range catalog {
		lm := catalog[i]
		if onlyProv != "" && lm.Prov != onlyProv {
			continue
		}
		if onlyProv == "" && firstPage && lm.Prov == config.ProviderNameChatGPTSub {
			continue
		}
		if skip[lmKey(lm)] {
			continue
		}
		out = append(out, lm)
		skip[lmKey(lm)] = true
		if len(out) >= need {
			break
		}
	}
	return out
}

func printPickerMsg(out io.Writer, msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" || out == nil {
		return
	}
	fmt.Fprintln(out, termcolor.WrapSystem(msg))
}

func SlashModels(d Deps) error {
	ctx := d.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	catalog, byProv, err := fetchSlashModelCatalogCached(ctx, d)
	if err != nil {
		return err
	}
	if len(catalog) == 0 {
		return fmt.Errorf("no models available")
	}

	pick := &slashModelPickerCtx{
		d:               d,
		fullCatalog:     catalog,
		providerCatalog: byProv,
		displayedKeys:   map[string]bool{},
		indexTable:      map[int]pickerRow{},
	}
	if pick.providerCatalog == nil {
		pick.providerCatalog = map[string][]ListedModel{}
	}
	for {
		display, hasMore := pick.buildDisplay()
		var pickerBuf bytes.Buffer
		printSlashModelPickerDisplay(&pickerBuf, display)
		if hasMore {
			fmt.Fprintln(&pickerBuf, "...")
		}
		fmt.Fprintln(&pickerBuf)
		writeSlashModelPickerHelp(&pickerBuf, pick.maxIndex(), hasMore, pick.filterProv)
		termcolor.WriteSystem(d.Out, pickerBuf.String())

		line, err := readSlashModelInput(d)
		if err != nil {
			return err
		}
		if line == "" {
			printPickerMsg(d.Out, "Invalid: empty input.")
			continue
		}
		if line == slashModelMoreCmd {
			if !hasMore {
				printPickerMsg(d.Out, "Invalid: no more models to show.")
				continue
			}
			pick.page++
			continue
		}
		if strings.EqualFold(line, "all") {
			if pick.filterProv == "" && pick.page == 0 {
				printPickerMsg(d.Out, "Invalid: already showing all providers.")
				continue
			}
			pick.filterProv = ""
			pick.resetView()
			continue
		}
		if prov, ok := pick.resolveProviderFilter(line); ok {
			if prov == pick.filterProv && pick.page == 0 {
				printPickerMsg(d.Out, fmt.Sprintf("Invalid: already filtered to %s.", prov))
				continue
			}
			if err := pick.ensureProviderCatalog(ctx, d.Cfg, prov); err != nil {
				printPickerMsg(d.Out, fmt.Sprintf("provider %s: error: %v", prov, err))
				continue
			}
			pick.filterProv = prov
			pick.resetView()
			continue
		}
		ok, msg, ferr := trySlashModelPick(d, pick, pick.filteredCatalog(), line)
		if ferr != nil {
			return ferr
		}
		if ok {
			PrintSystemf(d.Out, "Using %s[%s]", d.Model(), d.Provider().Name)
			return nil
		}
		printPickerMsg(d.Out, fmt.Sprintf("Invalid: %s", msg))
	}
}

func readSlashModelInput(d Deps) (string, error) {
	return config.ReadPromptLine(PromptIO(d), "> ")
}

func writeSlashModelPickerHelp(out io.Writer, lastIdx int, hasMore bool, filterProv string) {
	var b strings.Builder
	fmt.Fprintf(&b, "Select: index 0-%d, paste exact model id", lastIdx)
	if hasMore {
		b.WriteString(", > for next page")
	}
	b.WriteString(", provider name to filter")
	if filterProv != "" {
		fmt.Fprintf(&b, " (filtered: %s, all models; type all to reset)", filterProv)
	}
	fmt.Fprintln(out, b.String())
}

func printSlashModelPickerDisplay(out io.Writer, display []slashPickerDisplayRow) {
	if len(display) == 0 {
		return
	}
	rows := make([]pickerRow, len(display))
	for i := range display {
		rows[i] = display[i].pr
	}
	idxColW := pickerIndexColWidthForMax(displayMaxIndex(display))
	modelColW := pickerMaxModelLen(rows)
	provColW := pickerMaxProvBracketLen(rows)
	tagColW := pickerMaxTagLen(rows)

	printedRecents := false
	printedClaude := false
	printedModels := false
	for _, dr := range display {
		switch dr.pr.section {
		case sectionRecent:
			if !printedRecents {
				fmt.Fprintln(out, "[recents]")
				printedRecents = true
			}
		case sectionClaudeSub:
			if !printedClaude {
				fmt.Fprintln(out, "[Claude Sub]")
				printedClaude = true
			}
		case sectionCatalog:
			if !printedModels {
				fmt.Fprintln(out, "[models]")
				printedModels = true
			}
		}
		tag := dr.pr.displayTag()
		if dr.pr.section == sectionCatalog {
			tag = ""
		}
		writePickerModelLine(out, dr.index, idxColW, modelColW, provColW, tagColW, dr.pr.lm.Model, dr.pr.lm.Prov, tag)
	}
}

func displayMaxIndex(display []slashPickerDisplayRow) int {
	max := 0
	for _, dr := range display {
		if dr.index > max {
			max = dr.index
		}
	}
	return max
}

func pickerIndexColWidthForMax(maxIdx int) int {
	return pickerIndexColWidth(maxIdx + 1)
}

func trySlashModelPick(d Deps, pick *slashModelPickerCtx, catalog []ListedModel, line string) (ok bool, errMsg string, err error) {
	if config.AllDigits(line) {
		n, ierr := strconv.Atoi(line)
		if ierr != nil {
			return false, "not a valid number.", nil
		}
		pr, found := pick.indexTable[n]
		if !found {
			return false, fmt.Sprintf("index must be between 0 and %d.", pick.maxIndex()), nil
		}
		if aerr := d.ApplyCurrentModel(pr.lm.Prov, pr.lm.Model); aerr != nil {
			return false, aerr.Error(), nil
		}
		return true, "", nil
	}
	if rerr := resolveModelPaste(d, catalog, line); rerr != nil {
		return false, rerr.Error(), nil
	}
	return true, "", nil
}

func resolveModelPaste(d Deps, rows []ListedModel, id string) error {
	var matches []ListedModel
	for _, row := range rows {
		if row.Model == id {
			matches = append(matches, row)
		}
	}
	if len(matches) == 0 {
		return fmt.Errorf("model id %q not in the listed models", id)
	}
	if len(matches) > 1 {
		return fmt.Errorf("model id %q exists for multiple providers; use the numeric index from /models", id)
	}
	return d.ApplyCurrentModel(matches[0].Prov, matches[0].Model)
}
