package kit

import (
	"reflect"
	"testing"
)

// Every key in the registry names a key or what it shows, and a group never
// gives two of its keys the same name in the footer.
func TestKeys(t *testing.T) {
	groups := map[string]any{"ListKeys": ListKeys, "ChatKeys": ChatKeys, "TerminalKeys": TerminalKeys, "GitKeys": GitKeys,
		"SettingsKeys": SettingsKeys, "WorkspaceKeys": WorkspaceKeys, "StartKeys": StartKeys}
	for name, g := range groups {
		v := reflect.ValueOf(g)
		for i := range v.NumField() {
			k := v.Field(i).Interface().(Key)
			field := name + "." + v.Type().Field(i).Name
			if len(k.Keys) == 0 && k.Hint.Key == "" {
				t.Errorf("%s names no key", field)
			}
			if k.Hint.Key != "" && k.Hint.Does == "" {
				t.Errorf("%s shows a key doing nothing", field)
			}
		}
	}
}
