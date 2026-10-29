package config

import "testing"

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := LoadFrom(envMap(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != DefaultPort || cfg.CUPSHost != DefaultCUPSHost || cfg.MaxUploadMB != DefaultMaxUploadMB {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.MaxUploadBytes() != DefaultMaxUploadMB<<20 {
		t.Fatalf("MaxUploadBytes = %d", cfg.MaxUploadBytes())
	}
	if cfg.Addr() != ":8080" {
		t.Fatalf("Addr = %q", cfg.Addr())
	}
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := LoadFrom(envMap(map[string]string{
		"PORT":            "9090",
		"CUPS_HOST":       "cups:631",
		"DEFAULT_PRINTER": "HP_DeskJet_2300",
		"MAX_UPLOAD_MB":   "50",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "9090" || cfg.CUPSHost != "cups:631" || cfg.DefaultPrinter != "HP_DeskJet_2300" || cfg.MaxUploadMB != 50 {
		t.Fatalf("overrides not applied: %+v", cfg)
	}
}

func TestLoadRejectsInvalid(t *testing.T) {
	cases := map[string]map[string]string{
		"port not numeric":    {"PORT": "http"},
		"port out of range":   {"PORT": "70000"},
		"host flag injection": {"CUPS_HOST": "-oFoo"},
		"host with spaces":    {"CUPS_HOST": "cups host"},
		"printer with slash":  {"DEFAULT_PRINTER": "../etc"},
		"printer flag":        {"DEFAULT_PRINTER": "-d"},
		"upload zero":         {"MAX_UPLOAD_MB": "0"},
		"upload too large":    {"MAX_UPLOAD_MB": "100000"},
		"upload not numeric":  {"MAX_UPLOAD_MB": "ten"},
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadFrom(envMap(env)); err == nil {
				t.Fatalf("expected error for %v", env)
			}
		})
	}
}
