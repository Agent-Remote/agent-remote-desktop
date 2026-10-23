package app

import (
	"encoding/json"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) accountCreateForm(c *ui.Context) {
	p := &a.account
	ui.Text(c, locales.AccountCreateHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
	for _, row := range [][]struct {
		label string
		value *string
	}{
		{{locales.AccountFieldTool(), &p.form.Tool}, {locales.AccountFieldName(), &p.form.Name}},
		{{locales.AccountFieldRegion(), &p.form.Region}, {locales.AccountFieldTimezone(), &p.form.Timezone}},
		{{locales.AccountFieldLocale(), &p.form.Locale}, {locales.AccountFieldTags(), &p.tags}},
	} {
		ui.Row(c).FillWidth().Gap(12).Children(func() {
			for _, field := range row {
				ui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
					ui.Text(c, locales.AccountFieldValue(field.label, "")).FontSize(12)
					ui.TextInput(c, field.value).Label(field.label).Placeholder(field.label).FillWidth()
				})
			}
		})
	}
	ui.Text(c, locales.AccountTagsHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
}
func (a *App) reviewAccountCreate() {
	p := &a.account
	if p.submitted || p.loading || !a.accountOperationAvailable("accounts.create") {
		return
	}
	var tags []string
	if json.Unmarshal([]byte(p.tags), &tags) != nil {
		p.err = fault.AccountInputInvalid
		return
	}
	form := p.form
	form.Tags = tags
	if err := remote.ValidateAccountCreate(form); err != nil {
		p.err = err
		return
	}
	p.err = nil
	p.form = form
	p.review = true
}
func (a *App) accountCreateReview(c *ui.Context) {
	p := &a.account
	for _, field := range []struct{ label, value string }{
		{locales.AccountFieldTool(), p.form.Tool}, {locales.AccountFieldName(), p.form.Name}, {locales.AccountFieldRegion(), p.form.Region}, {locales.AccountFieldTimezone(), p.form.Timezone}, {locales.AccountFieldLocale(), p.form.Locale},
	} {
		ui.Text(c, locales.AccountFieldValue(field.label, field.value))
	}
	tags := locales.AccountNoTags()
	if len(p.form.Tags) > 0 {
		tags = strings.Join(p.form.Tags, ", ")
	}
	ui.Text(c, locales.AccountFieldValue(locales.AccountFieldTags(), tags))
}
