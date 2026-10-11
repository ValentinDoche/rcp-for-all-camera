package gui

import (
	"errors"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/texts"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
	"github.com/valentindoche/rcp-for-all-camera/internal/i18n"
)

// maxImportSize is the largest configuration file the import reads.
const maxImportSize = 1 << 20

// configEnv is what the Configuration page needs from the window.
type configEnv struct {
	win     fyne.Window
	api     SettingsAPI
	mode    style.Mode
	setMode func(style.Mode)
	lang    string
	langs   []i18n.Lang
	setLang func(code string)
	openURL func(*url.URL) error
	setTab  func(tab int)
}

// settingRow is one setting: label above the control, help text below, and
// an inline error line that only shows when needed.
type settingRow struct {
	obj  fyne.CanvasObject
	help *widget.RichText
	err  *widgets.Text
}

func newHelp(s string, tok fyne.ThemeColorName) *widget.RichText {
	h := widget.NewRichText()
	h.Wrapping = fyne.TextWrapWord
	setHelp(h, s, tok)
	return h
}

func setHelp(h *widget.RichText, s string, tok fyne.ThemeColorName) {
	h.Segments = []widget.RichTextSegment{&widget.TextSegment{Text: s,
		Style: widget.RichTextStyle{ColorName: tok, SizeName: theme.SizeNameCaptionText}}}
	h.Refresh()
}

func newSettingRow(label string, control fyne.CanvasObject, help string) *settingRow {
	r := &settingRow{help: newHelp(help, style.TextMuted), err: widgets.NewText("", style.Danger, 12).Bold()}
	r.err.Hide()
	r.obj = container.NewVBox(
		widgets.NewText(label, style.Text, 13).Bold(),
		control,
		r.help,
		r.err,
	)
	return r
}

// setError shows msg under the setting, or hides the line when empty.
func (r *settingRow) setError(msg string) {
	r.err.SetText(msg)
	if msg == "" {
		r.err.Hide()
	} else {
		r.err.Show()
	}
}

// generalTab is Configuration > General. Each setting has its own small
// builder (see groups); add a setting by writing one and listing it there.
type generalTab struct {
	env *configEnv
	ctl *Controller
	tx  texts.Texts
	obj fyne.CanvasObject

	quiet bool

	nic     *widget.Select
	nicIPs  []string // IP of each option, "" = default
	nicRow  *settingRow
	nicPrev int

	langSeg, themeSeg        *widgets.Segmented
	autoSeg, lockSeg, logSeg *widgets.Segmented
	autoRow, lockRow, logRow *settingRow
	exportRow, importRow     *settingRow
}

func newGeneralTab(ctl *Controller, env *configEnv) *generalTab {
	g := &generalTab{env: env, ctl: ctl, tx: ctl.Tx(), nicPrev: -1}

	type group struct {
		title string
		rows  []*settingRow
	}
	groups := []group{
		{"config.general.interface", []*settingRow{g.languageRow(), g.themeRow()}},
		{"config.general.network", []*settingRow{g.networkRow()}},
		// Room for a future "panel brightness" setting in its own group,
		// once the hardware is known to support it.
		{"config.general.system", []*settingRow{g.autoStartRow(), g.lockSetting(), g.logsRow()}},
		{"config.general.data", []*settingRow{g.exportSetting(), g.importSetting()}},
	}
	var blocks []fyne.CanvasObject
	for _, gr := range groups {
		body := container.NewVBox(
			widgets.NewText(g.tx.T(gr.title), style.Text, 14).Bold(),
			widgets.Sized(0, 1, widgets.NewHairline()),
		)
		for _, r := range gr.rows {
			body.Add(container.New(layout.NewCustomPaddedLayout(4, 4, 0, 0), r.obj))
		}
		blocks = append(blocks, widgets.NewPanel(style.Surface, style.Border, 12, body))
	}
	col := widgets.Sized(680, 0, container.NewVBox(spaced(blocks)...))
	g.obj = container.NewVScroll(container.New(layout.NewCustomPaddedLayout(0, 8, 0, 8),
		container.NewHBox(layout.NewSpacer(), col, layout.NewSpacer())))
	g.Refresh()
	return g
}

// Refresh re-reads the settings and the network adapters; it runs each time
// the tab opens.
func (g *generalTab) Refresh() {
	st := g.env.api.Settings()
	g.quiet = true
	defer func() { g.quiet = false }()
	g.refreshNIC(st)
	g.autoSeg.SetSelected(yesNo(st.AutoStart))
	g.lockSeg.SetSelected(yesNo(st.OperatorLock))
	if st.LogLevel == app.LogDebug {
		g.logSeg.SetSelected(1)
	} else {
		g.logSeg.SetSelected(0)
	}
}

func yesNo(on bool) int {
	if on {
		return 0
	}
	return 1
}

func (g *generalTab) yesNoSeg(onChange func(on bool)) *widgets.Segmented {
	return widgets.NewSegmented([]string{g.tx.T("config.general.yes"), g.tx.T("config.general.no")}, func(i int) {
		if !g.quiet {
			onChange(i == 0)
		}
	})
}

// --- one builder per setting --------------------------------------------------

func (g *generalTab) languageRow() *settingRow {
	names := make([]string, len(g.env.langs))
	sel := -1
	for i, l := range g.env.langs {
		names[i] = l.Name
		if l.Code == g.env.lang {
			sel = i
		}
	}
	seg := widgets.NewSegmented(names, func(i int) { g.env.setLang(g.env.langs[i].Code) })
	seg.SetSelected(sel)
	g.langSeg = seg
	return newSettingRow(g.tx.T("config.general.language"), container.NewHBox(seg), g.tx.T("config.general.language.help"))
}

func (g *generalTab) themeRow() *settingRow {
	modes := []style.Mode{style.ModeSystem, style.ModeLight, style.ModeDark}
	seg := widgets.NewSegmented([]string{
		g.tx.T("config.general.theme.system"), g.tx.T("config.general.theme.light"), g.tx.T("config.general.theme.dark"),
	}, func(i int) {
		g.env.mode = modes[i]
		if g.env.setMode != nil {
			g.env.setMode(modes[i])
		}
	})
	for i, m := range modes {
		if m == g.env.mode {
			seg.SetSelected(i)
		}
	}
	g.themeSeg = seg
	return newSettingRow(g.tx.T("config.general.theme"), container.NewHBox(seg), g.tx.T("config.general.theme.help"))
}

func (g *generalTab) networkRow() *settingRow {
	g.nic = widget.NewSelect(nil, g.onNIC)
	g.nicRow = newSettingRow(g.tx.T("config.general.nic"), widgets.Sized(360, 0, g.nic), g.tx.T("config.general.nic.help"))
	return g.nicRow
}

func (g *generalTab) refreshNIC(st app.Settings) {
	opts := []string{g.tx.T("config.general.nic.default")}
	ips := []string{""}
	sel := 0
	for _, n := range g.env.api.NetworkInterfaces() {
		opts = append(opts, n.Name+" : "+n.IP)
		ips = append(ips, n.IP)
		if n.IP == st.BindIP {
			sel = len(opts) - 1
		}
	}
	if st.BindIP != "" && !st.BindIPAvailable {
		opts = append(opts, g.tx.T("config.general.nic.missing", "ip", st.BindIP))
		ips = append(ips, st.BindIP)
		sel = len(opts) - 1
		setHelp(g.nicRow.help, g.tx.T("config.general.nic.warning"), style.Warn)
	} else {
		setHelp(g.nicRow.help, g.tx.T("config.general.nic.help"), style.TextMuted)
	}
	g.nicIPs = ips
	g.nic.Options = opts
	prev := g.quiet
	g.quiet = true
	g.nic.SetSelectedIndex(sel)
	g.quiet = prev
	g.nicPrev = sel
}

func (g *generalTab) onNIC(string) {
	if g.quiet {
		return
	}
	i := g.nic.SelectedIndex()
	if i < 0 || i >= len(g.nicIPs) {
		return
	}
	if err := g.env.api.SetBindIP(g.nicIPs[i]); err != nil {
		g.nicRow.setError(g.tx.BindError(err))
		g.quiet = true
		g.nic.SetSelectedIndex(g.nicPrev)
		g.quiet = false
		return
	}
	g.nicRow.setError("")
	g.nicPrev = i
	g.refreshNIC(g.env.api.Settings())
}

func (g *generalTab) autoStartRow() *settingRow {
	g.autoSeg = g.yesNoSeg(func(on bool) {
		if err := g.env.api.SetAutoStart(on); err != nil {
			g.autoRow.setError(g.tx.AutoStartError(err))
			g.revert(g.autoSeg, !on)
			return
		}
		g.autoRow.setError("")
	})
	g.autoRow = newSettingRow(g.tx.T("config.general.autostart"), container.NewHBox(g.autoSeg), g.tx.T("config.general.autostart.help"))
	return g.autoRow
}

func (g *generalTab) lockSetting() *settingRow {
	g.lockSeg = g.yesNoSeg(func(on bool) {
		if err := g.env.api.SetOperatorLock(on); err != nil {
			g.lockRow.setError(g.tx.ConfigError(err))
			g.revert(g.lockSeg, !on)
			return
		}
		g.lockRow.setError("")
	})
	g.lockRow = newSettingRow(g.tx.T("config.general.lock"), container.NewHBox(g.lockSeg), g.tx.T("config.general.lock.help"))
	return g.lockRow
}

func (g *generalTab) logsRow() *settingRow {
	g.logSeg = widgets.NewSegmented([]string{g.tx.T("config.general.logs.normal"), g.tx.T("config.general.logs.debug")}, func(i int) {
		if g.quiet {
			return
		}
		level := app.LogNormal
		if i == 1 {
			level = app.LogDebug
		}
		if err := g.env.api.SetLogLevel(level); err != nil {
			g.logRow.setError(g.tx.ConfigError(err))
			g.quiet = true
			g.logSeg.SetSelected(1 - i)
			g.quiet = false
			return
		}
		g.logRow.setError("")
	})
	open := widget.NewButtonWithIcon(g.tx.T("config.general.logs.open"), theme.FolderOpenIcon(), g.openLogs)
	g.logRow = newSettingRow(g.tx.T("config.general.logs"),
		container.NewHBox(g.logSeg, widgets.Sized(8, 0, layout.NewSpacer()), open), g.tx.T("config.general.logs.help"))
	return g.logRow
}

// revert puts a yes/no control back to the state it had before a failed call.
func (g *generalTab) revert(seg *widgets.Segmented, on bool) {
	seg.SetSelected(yesNo(on))
}

// logDirURL builds the file URL of a folder without any shell involved.
func logDirURL(dir string) *url.URL {
	p := filepath.ToSlash(dir)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p // C:/x becomes /C:/x so the URL is file:///C:/x
	}
	return &url.URL{Scheme: "file", Path: p}
}

func (g *generalTab) openLogs() {
	dir := g.env.api.Settings().LogDir
	if dir == "" || g.env.openURL == nil || g.env.openURL(logDirURL(dir)) != nil {
		g.logRow.setError(g.tx.T("config.general.logs.openFailed"))
		return
	}
	g.logRow.setError("")
}

// --- export and import --------------------------------------------------------

func (g *generalTab) exportSetting() *settingRow {
	btn := widget.NewButtonWithIcon(g.tx.T("config.general.export.button"), theme.DocumentSaveIcon(), func() {
		d := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
			if err != nil || w == nil {
				return
			}
			g.exportTo(w)
		}, g.env.win)
		d.SetFileName("rcp-config.json")
		d.SetFilter(storage.NewExtensionFileFilter([]string{".json"}))
		d.Show()
	})
	g.exportRow = newSettingRow(g.tx.T("config.general.export"), container.NewHBox(btn), g.tx.T("config.general.export.help"))
	return g.exportRow
}

// exportTo writes the configuration document to w and closes it.
func (g *generalTab) exportTo(w io.WriteCloser) {
	defer w.Close()
	b, err := g.env.api.ExportConfig()
	if err == nil {
		_, err = w.Write(b)
	}
	if err != nil {
		g.exportRow.setError(g.tx.ConfigError(err))
		return
	}
	g.exportRow.setError("")
	g.ctl.SetNotice(texts.Notice{Text: g.tx.T("config.general.export.done"), Level: texts.LevelInfo})
}

func (g *generalTab) importSetting() *settingRow {
	btn := widget.NewButtonWithIcon(g.tx.T("config.general.import.button"), theme.FolderOpenIcon(), func() {
		d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			defer r.Close()
			g.importFrom(r)
		}, g.env.win)
		d.SetFilter(storage.NewExtensionFileFilter([]string{".json"}))
		d.Show()
	})
	g.importRow = newSettingRow(g.tx.T("config.general.import"), container.NewHBox(btn), g.tx.T("config.general.import.help"))
	return g.importRow
}

// importFrom reads at most 1 MiB, then asks before replacing the rig.
func (g *generalTab) importFrom(r io.Reader) {
	data, err := io.ReadAll(io.LimitReader(r, maxImportSize+1))
	switch {
	case err != nil:
		g.importRow.setError(g.tx.T("config.general.import.readFailed"))
		return
	case len(data) > maxImportSize:
		g.importRow.setError(g.tx.T("config.general.import.tooLarge"))
		return
	}
	g.importRow.setError("")
	showConfirm(g.env.win, g.tx.T("config.general.import.title"), g.tx.T("config.general.import.body"),
		g.tx.T("config.general.import.yes"), g.tx.T("confirm.no"), func(ok bool) {
			if ok {
				g.applyImport(data)
			}
		})
}

// applyImport hands the document to the service; a refused file is reported
// inline and nothing changes.
func (g *generalTab) applyImport(data []byte) {
	if err := g.env.api.ImportConfig(data); err != nil {
		msg := g.tx.ConfigError(err)
		if IsAPIError(err, app.CodeInvalidConfig) {
			msg = g.tx.T("config.error.import", "detail", errDetail(err))
		}
		g.importRow.setError(msg)
		return
	}
	g.importRow.setError("")
	g.ctl.RefreshRig()
	g.ctl.SetNotice(texts.Notice{Text: g.tx.T("config.general.import.done"), Level: texts.LevelInfo})
}

func errDetail(err error) string {
	var ae *app.Error
	if errors.As(err, &ae) {
		return ae.Message
	}
	return ""
}
