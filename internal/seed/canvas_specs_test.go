package seed_test

import (
	"encoding/json"
	"testing"

	"github.com/MajestaNet/ide/internal/canvas"
	"github.com/MajestaNet/ide/internal/packages"
	_ "github.com/MajestaNet/ide/internal/seed"
)

func TestSalesAndServiceCanvasSpecTemplatesValidate(t *testing.T) {
	sales, ok := packages.Get("sales")
	if !ok {
		t.Fatal("sales module missing")
	}
	if len(sales.CanvasSpecTemplates) == 0 {
		t.Fatal("sales should ship CanvasSpecTemplates")
	}
	service, ok := packages.Get("service")
	if !ok {
		t.Fatal("service module missing")
	}
	if len(service.CanvasSpecTemplates) == 0 {
		t.Fatal("service should ship CanvasSpecTemplates")
	}
	for _, m := range []packages.Module{sales, service} {
		for _, tpl := range m.CanvasSpecTemplates {
			layout := json.RawMessage(tpl.LayoutJSON)
			nodes := json.RawMessage(tpl.NodesJSON)
			bindings := json.RawMessage(`[]`)
			if tpl.BindingsJSON != "" {
				bindings = json.RawMessage(tpl.BindingsJSON)
			}
			if err := canvas.ValidateSpecBody(layout, nodes, bindings); err != nil {
				t.Fatalf("%s/%s: %v", m.Name, tpl.APIName, err)
			}
		}
	}
}
