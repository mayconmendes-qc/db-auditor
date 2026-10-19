package pii

import (
	"strings"
	"unicode"
)

type DictionaryEntry struct {
	Pattern  string `json:"name_pattern"`
	Category string `json:"category"`
	Active   bool   `json:"active"`
}

func DefaultDictionary() []DictionaryEntry {
	return []DictionaryEntry{
		{Pattern: "cpf", Category: CatCPF, Active: true},
		{Pattern: "cpf_numero", Category: CatCPF, Active: true},
		{Pattern: "nr_cpf", Category: CatCPF, Active: true},
		{Pattern: "tax_id", Category: CatCPF, Active: true},
		{Pattern: "cnpj", Category: CatCNPJ, Active: true},
		{Pattern: "cnpj_numero", Category: CatCNPJ, Active: true},
		{Pattern: "nr_cnpj", Category: CatCNPJ, Active: true},
		{Pattern: "email", Category: CatEmail, Active: true},
		{Pattern: "e_mail", Category: CatEmail, Active: true},
		{Pattern: "mail", Category: CatEmail, Active: true},
		{Pattern: "email_address", Category: CatEmail, Active: true},
		{Pattern: "telefone", Category: CatTelefone, Active: true},
		{Pattern: "celular", Category: CatTelefone, Active: true},
		{Pattern: "fone", Category: CatTelefone, Active: true},
		{Pattern: "phone", Category: CatTelefone, Active: true},
		{Pattern: "mobile", Category: CatTelefone, Active: true},
		{Pattern: "cep", Category: CatCEP, Active: true},
		{Pattern: "zipcode", Category: CatCEP, Active: true},
		{Pattern: "zip_code", Category: CatCEP, Active: true},
		{Pattern: "pix", Category: CatPIX, Active: true},
		{Pattern: "chave_pix", Category: CatPIX, Active: true},
		{Pattern: "pix_key", Category: CatPIX, Active: true},
		{Pattern: "nome", Category: CatNome, Active: true},
		{Pattern: "nome_completo", Category: CatNome, Active: true},
		{Pattern: "full_name", Category: CatNome, Active: true},
		{Pattern: "nascimento", Category: CatNascimento, Active: true},
		{Pattern: "data_nascimento", Category: CatNascimento, Active: true},
		{Pattern: "birth_date", Category: CatNascimento, Active: true},
		{Pattern: "endereco", Category: CatEndereco, Active: true},
		{Pattern: "logradouro", Category: CatEndereco, Active: true},
		{Pattern: "address", Category: CatEndereco, Active: true},
		{Pattern: "rg", Category: CatRG, Active: true},
		{Pattern: "rg_numero", Category: CatRG, Active: true},
	}
}

func MatchColumn(columnName string, entries []DictionaryEntry) (category string, ok bool) {
	norm := normalizeColumnName(columnName)
	if norm == "" {
		return "", false
	}
	bestLen := 0
	bestExact := false
	for _, e := range entries {
		if !e.Active {
			continue
		}
		pat := normalizeColumnName(e.Pattern)
		if pat == "" {
			continue
		}
		exact := norm == pat || hasToken(norm, pat)
		sub := strings.Contains(norm, pat)
		if !exact && !sub {
			continue
		}
		if exact && (!bestExact || len(pat) > bestLen) {
			category, ok, bestLen, bestExact = e.Category, true, len(pat), true
			continue
		}
		if !exact && !bestExact && len(pat) > bestLen {
			category, ok, bestLen = e.Category, true, len(pat)
		}
	}
	return category, ok
}

func normalizeColumnName(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	prev := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prev = false
			continue
		}
		if !prev {
			b.WriteByte('_')
			prev = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func hasToken(name, pattern string) bool {
	for _, p := range strings.Split(name, "_") {
		if p == pattern {
			return true
		}
	}
	return false
}

func ValidCategory(category string) bool {
	for _, c := range AllCategories {
		if c == category {
			return true
		}
	}
	return false
}
