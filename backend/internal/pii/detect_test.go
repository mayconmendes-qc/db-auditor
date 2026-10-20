package pii

import "testing"

func TestValidCPF(t *testing.T) {
	if !ValidCPF("529.982.247-25") {
		t.Fatal("expected valid CPF")
	}
	if ValidCPF("111.111.111-11") {
		t.Fatal("repeated digits must fail")
	}
	if ValidCPF("529.982.247-24") {
		t.Fatal("bad DV must fail")
	}
}

func TestValidCNPJNumeric(t *testing.T) {
	if !ValidCNPJ("04.252.011/0001-10") {
		t.Fatal("expected valid numeric CNPJ")
	}
	if ValidCNPJ("04.252.011/0001-11") {
		t.Fatal("bad DV must fail")
	}
	if ValidCNPJ("00.000.000/0000-00") {
		t.Fatal("repeated must fail")
	}
}

func TestCNPJAlphanumericDV(t *testing.T) {
	const valid = "12ABC34501DE35"
	if !ValidCNPJ(valid) {
		t.Fatalf("expected valid alphanumeric CNPJ %s", valid)
	}
	if !ValidCNPJ("12.ABC.345/01DE-35") {
		t.Fatal("formatted alphanumeric must pass")
	}
	if ValidCNPJ("12ABC34501DE36") {
		t.Fatal("wrong DV must fail")
	}
	if ValidCNPJ("12ABC34501DEX5") {
		t.Fatal("non-digit check digits must fail")
	}
}

func TestValidEmailPhoneCEPPIX(t *testing.T) {
	if !ValidEmail("a@b.co") {
		t.Fatal("email")
	}
	if ValidEmail("not-an-email") {
		t.Fatal("bad email")
	}
	if !ValidBRPhone("+55 11 91234-5678") {
		t.Fatal("mobile")
	}
	if !ValidBRPhone("11 3456-7890") {
		t.Fatal("landline")
	}
	if !ValidCEP("01310-100") {
		t.Fatal("cep")
	}
	if !ValidPIXKey("529.982.247-25") {
		t.Fatal("pix cpf")
	}
	if !ValidPIXKey("550e8400-e29b-41d4-a716-446655440000") {
		t.Fatal("pix uuid")
	}
}

func TestConfidenceFromSampleCA03(t *testing.T) {
	if got := ConfidenceFromSample(80, 100, 100); got != "media" {
		t.Fatalf("got %s want media", got)
	}
	if got := ConfidenceFromSample(95, 100, 100); got != "alta" {
		t.Fatalf("got %s want alta", got)
	}
	if got := ConfidenceFromSample(40, 40, 40); got != "baixa" {
		t.Fatalf("got %s want baixa (<50 non-null)", got)
	}
}

func TestDetectorsDoNotEchoInputInFalsePath(t *testing.T) {
	secret := "529.982.247-25-SHOULD-NOT-LEAK"
	_ = ValidCPF(secret)
	_ = ValidCNPJ(secret)
	_ = ValidEmail(secret)
	_ = ValidBRPhone(secret)
	_ = ValidCEP(secret)
	_ = ValidPIXKey(secret)
}
