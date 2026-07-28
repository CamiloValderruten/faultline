package discordgo

import "testing"

func TestDAVESessionGeneratesGatewayKeyPackage(t *testing.T) {
	d, err := NewDAVESession("1531759252501168158", "1531760188787392566", 1)
	if err != nil {
		t.Fatalf("NewDAVESession returned error: %v", err)
	}
	keyPackage, err := d.GenerateKeyPackage()
	if err != nil {
		t.Fatalf("GenerateKeyPackage returned error: %v", err)
	}
	if len(keyPackage) == 0 {
		t.Fatal("expected a non-empty gateway key package")
	}
	if d.CanEncrypt() {
		t.Fatal("expected DAVE to remain pending before commit or Welcome")
	}
}
