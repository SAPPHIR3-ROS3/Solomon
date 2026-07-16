package commands

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands/connect"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

const (
	slashModelMoreCmd           = ">"
	slashPickerRecentSlots      = 10
	slashPickerIdxChatGPTSub    = 11
	slashPickerIdxClaudeSub     = 12
	slashPickerIdxCursorAPI     = 13
	slashPickerIdxOtherStart    = 14
)

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
	includeCurrent      bool
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
	return c.loadProviderCatalog(ctx, cfg, prov, false)
}

func (c *slashModelPickerCtx) loadProviderCatalog(ctx context.Context, cfg *config.Root, prov string, forceRefresh bool) error {
	if c.providerCatalog == nil {
		c.providerCatalog = map[string][]ListedModel{}
	}
	if !forceRefresh {
		if _, ok := c.providerCatalog[prov]; ok {
			return nil
		}
		if cat, ok := cachedProviderCatalogSlice(cfg, prov); ok {
			c.providerCatalog[prov] = cat
			return nil
		}
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
		if rows[i].isProvider() {
			continue
		}
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
	if c.includeCurrent {
		n--
	}
	if n < 1 {
		return 1
	}
	return n
}

func (c *slashModelPickerCtx) providerPageNeed(catalogLen int) int {
	if c.page > 0 {
		return c.catalogPageSize()
	}
	cap := config.ModelPickerPageCap(catalogLen)
	if cap >= catalogLen {
		return catalogLen
	}
	if c.includeCurrent && strings.TrimSpace(c.cur().Model) != "" && cap > 0 {
		return cap - 1
	}
	return cap
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
	max := -1
	for idx := range c.indexTable {
		if idx > max {
			max = idx
		}
	}
	if max < 0 {
		return 0
	}
	return max
}

func (c *slashModelPickerCtx) buildDisplay() ([]slashPickerDisplayRow, bool) {
	if c.page == 0 {
		return c.buildFirstPage()
	}
	return c.buildNextPage()
}

func (c *slashModelPickerCtx) buildFirstPage() ([]slashPickerDisplayRow, bool) {
	if c.providerFilterActive() {
		return c.buildProviderUnifiedPage()
	}
	catalog := c.filteredCatalog()
	cur := c.cur()
	claimed := map[string]bool{lmKey(cur): true}
	recents := pickRecentListed(c.d, catalog, cur, claimed, slashPickerRecentSlots)
	c.stickyRecents = recents
	c.firstPageRecents = len(recents)
	c.indexTable = map[int]pickerRow{}
	c.nextIndex = 0

	if c.includeCurrent && strings.TrimSpace(cur.Prov) != "" && strings.TrimSpace(cur.Model) != "" {
		c.indexTable[0] = pickerRow{lm: cur, section: sectionCurrent}
	}
	for i := range recents {
		c.indexTable[1+i] = pickerRow{lm: recents[i], section: sectionRecent}
	}

	present := map[string]bool{}
	for _, p := range config.ProviderList(c.d.Cfg) {
		present[p.Name] = true
	}
	if present[config.ProviderNameChatGPTSub] {
		c.indexTable[slashPickerIdxChatGPTSub] = pickerRow{provOnly: config.ProviderNameChatGPTSub, section: sectionProvider}
	}
	if present[config.ProviderNameClaudeSub] {
		c.indexTable[slashPickerIdxClaudeSub] = pickerRow{provOnly: config.ProviderNameClaudeSub, section: sectionProvider}
	}
	if present[config.ProviderNameCursorAPI] {
		c.indexTable[slashPickerIdxCursorAPI] = pickerRow{provOnly: config.ProviderNameCursorAPI, section: sectionProvider}
	}

	others := make([]string, 0, len(present))
	for name := range present {
		switch name {
		case config.ProviderNameChatGPTSub, config.ProviderNameClaudeSub, config.ProviderNameCursorAPI:
			continue
		default:
			others = append(others, name)
		}
	}
	sort.Strings(others)
	idx := slashPickerIdxOtherStart
	for _, name := range others {
		c.indexTable[idx] = pickerRow{provOnly: name, section: sectionProvider}
		idx++
	}
	c.nextIndex = idx

	disp := c.rootPageDisplay()
	return disp, false
}

func (c *slashModelPickerCtx) rootPageDisplay() []slashPickerDisplayRow {
	var disp []slashPickerDisplayRow
	add := func(i int) {
		if pr, ok := c.indexTable[i]; ok {
			disp = append(disp, slashPickerDisplayRow{index: i, pr: pr})
		}
	}
	add(0)
	for i := 1; i <= slashPickerRecentSlots; i++ {
		add(i)
	}
	add(slashPickerIdxChatGPTSub)
	add(slashPickerIdxClaudeSub)
	add(slashPickerIdxCursorAPI)
	for i := slashPickerIdxOtherStart; i < c.nextIndex; i++ {
		add(i)
	}
	return disp
}

func (c *slashModelPickerCtx) providerRecentKeys() map[string]bool {
	out := map[string]bool{}
	if c.d.Cfg == nil || c.filterProv == "" {
		return out
	}
	for _, u := range config.RecentModelUseEntries(c.d.Cfg, c.filterProv) {
		lm := ListedModel{Prov: strings.TrimSpace(u.Provider), Model: strings.TrimSpace(u.Model)}
		if lm.Prov == "" || lm.Model == "" {
			continue
		}
		out[lmKey(lm)] = true
	}
	return out
}

func (c *slashModelPickerCtx) buildProviderUnifiedPage() ([]slashPickerDisplayRow, bool) {
	catalog := orderListedModelsByProvider(append([]ListedModel(nil), c.filteredCatalog()...), c.filterProv)
	cur := c.cur()
	recentKeys := c.providerRecentKeys()
	need := c.providerPageNeed(len(catalog))
	var batch []ListedModel
	for i := range catalog {
		lm := catalog[i]
		if lmKey(lm) == lmKey(cur) {
			continue
		}
		if c.displayedKeys[lmKey(lm)] {
			continue
		}
		batch = append(batch, lm)
		if len(batch) >= need {
			break
		}
	}
	c.lastPageCatalogFrom = c.nextIndex
	var batchRows []pickerRow
	for i := range batch {
		tag := ""
		if recentKeys[lmKey(batch[i])] {
			tag = "(recent)"
		}
		batchRows = append(batchRows, pickerRow{lm: batch[i], section: sectionCatalog, lineTag: tag})
	}
	if c.page == 0 {
		c.indexTable = map[int]pickerRow{}
		c.nextIndex = 0
		c.stickyRecents = nil
		c.firstPageRecents = 0
		if c.includeCurrent && strings.TrimSpace(cur.Prov) != "" && strings.TrimSpace(cur.Model) != "" {
			c.registerRow(pickerRow{lm: cur, section: sectionCurrent})
			c.displayedKeys[lmKey(cur)] = true
		}
	}
	c.registerRows(batchRows)
	c.markDisplayed(batchRows)

	var disp []slashPickerDisplayRow
	for idx := c.lastPageCatalogFrom; idx < c.nextIndex; idx++ {
		disp = append(disp, slashPickerDisplayRow{index: idx, pr: c.indexTable[idx]})
	}
	return disp, c.hasUndisplayed()
}

func (c *slashModelPickerCtx) buildNextPage() ([]slashPickerDisplayRow, bool) {
	if !c.providerFilterActive() {
		return nil, false
	}
	return c.buildProviderUnifiedPage()
}

func (c *slashModelPickerCtx) applyProviderFilter(ctx context.Context, prov string) error {
	if err := c.loadProviderCatalog(ctx, c.d.Cfg, prov, true); err != nil {
		return err
	}
	c.filterProv = prov
	c.resetView()
	return nil
}

func printPickerMsg(out io.Writer, msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" || out == nil {
		return
	}
	fmt.Fprintln(out, termcolor.WrapSystem(msg))
}

func SlashModels(d Deps) error {
	lm, err := PickListedModel(d, true)
	if err != nil {
		return err
	}
	if err := d.ApplyCurrentModel(lm.Prov, lm.Model); err != nil {
		return err
	}
	PrintSystemf(d.Out, "Using %s[%s]", d.Model(), d.Provider().Name)
	return nil
}

func PickListedModel(d Deps, includeCurrent bool) (ListedModel, error) {
	ctx := d.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	catalog, byProv, err := fetchSlashModelCatalogCached(ctx, d)
	if err != nil {
		return ListedModel{}, err
	}
	if len(catalog) == 0 {
		return ListedModel{}, fmt.Errorf("no models available")
	}

	pick := &slashModelPickerCtx{
		d:               d,
		fullCatalog:     catalog,
		providerCatalog: byProv,
		displayedKeys:   map[string]bool{},
		indexTable:      map[int]pickerRow{},
		includeCurrent:  includeCurrent,
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
			return ListedModel{}, err
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
			if err := pick.applyProviderFilter(ctx, prov); err != nil {
				printPickerMsg(d.Out, fmt.Sprintf("provider %s: error: %v", prov, err))
				continue
			}
			continue
		}
		lm, ok, msg, ferr := trySlashModelPickListed(pick, pick.filteredCatalog(), line)
		if ferr != nil {
			return ListedModel{}, ferr
		}
		if ok {
			if strings.TrimSpace(msg) == "provider" {
				if err := pick.applyProviderFilter(ctx, lm.Prov); err != nil {
					printPickerMsg(d.Out, fmt.Sprintf("provider %s: error: %v", lm.Prov, err))
					continue
				}
				continue
			}
			return lm, nil
		}
		printPickerMsg(d.Out, fmt.Sprintf("Invalid: %s", msg))
	}
}

func trySlashModelPickListed(pick *slashModelPickerCtx, catalog []ListedModel, line string) (ListedModel, bool, string, error) {
	if config.AllDigits(line) {
		n, ierr := strconv.Atoi(line)
		if ierr != nil {
			return ListedModel{}, false, "not a valid number.", nil
		}
		pr, found := pick.indexTable[n]
		if !found {
			return ListedModel{}, false, fmt.Sprintf("index must be between 0 and %d.", pick.maxIndex()), nil
		}
		if pr.isProvider() {
			return ListedModel{Prov: pr.provOnly}, true, "provider", nil
		}
		return pr.lm, true, "", nil
	}
	lm, rerr := resolveListedModelPaste(catalog, line)
	if rerr != nil {
		return ListedModel{}, false, rerr.Error(), nil
	}
	return lm, true, "", nil
}

func resolveListedModelPaste(rows []ListedModel, id string) (ListedModel, error) {
	var matches []ListedModel
	for _, row := range rows {
		if row.Model == id {
			matches = append(matches, row)
		}
	}
	if len(matches) == 0 {
		return ListedModel{}, fmt.Errorf("model id %q not in the listed models", id)
	}
	if len(matches) > 1 {
		return ListedModel{}, fmt.Errorf("model id %q exists for multiple providers; use the numeric index from /models", id)
	}
	return matches[0], nil
}

func readSlashModelInput(d Deps) (string, error) {
	return config.ReadPromptLine(PromptIO(d), "> ")
}
