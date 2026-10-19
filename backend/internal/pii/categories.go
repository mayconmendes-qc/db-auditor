package pii

const (
	CatCPF        = "cpf"
	CatCNPJ       = "cnpj"
	CatEmail      = "email"
	CatTelefone   = "telefone"
	CatCEP        = "cep"
	CatPIX        = "pix"
	CatNome       = "nome"
	CatNascimento = "nascimento"
	CatEndereco   = "endereco"
	CatRG         = "rg"
)

var AllCategories = []string{
	CatCPF, CatCNPJ, CatEmail, CatTelefone, CatCEP, CatPIX,
	CatNome, CatNascimento, CatEndereco, CatRG,
}

const (
	ConfidenceAlta  = "alta"
	ConfidenceMedia = "media"
	ConfidenceBaixa = "baixa"
)

const (
	MethodNome          = "nome"
	MethodConteudo      = "conteudo"
	MethodAmbos         = "ambos"
	MethodNaoVerificado = "nao_verificado"
)

func ContentDetectable(category string) bool {
	switch category {
	case CatCPF, CatCNPJ, CatEmail, CatTelefone, CatCEP, CatPIX:
		return true
	default:
		return false
	}
}

func SensitiveIdentity(category string) bool {
	switch category {
	case CatCPF, CatCNPJ, CatPIX:
		return true
	default:
		return false
	}
}

func CompatibleType(category, dataType string) bool {
	t := normalizeType(dataType)
	switch category {
	case CatCPF, CatCNPJ, CatRG, CatCEP, CatTelefone:
		return isTextual(t) || isNumeric(t)
	case CatEmail, CatPIX, CatNome, CatEndereco:
		return isTextual(t)
	case CatNascimento:
		return isTextual(t) || isDate(t)
	default:
		return false
	}
}

func normalizeType(dataType string) string {
	out := make([]rune, 0, len(dataType))
	for _, r := range dataType {
		if r >= 'A' && r <= 'Z' {
			out = append(out, r+('a'-'A'))
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func isTextual(t string) bool {
	return containsAny(t, "char", "text", "varchar", "citext", "name", "uuid")
}
func isNumeric(t string) bool {
	return containsAny(t, "int", "numeric", "decimal", "bigint", "smallint", "double", "real")
}
func isDate(t string) bool {
	return containsAny(t, "date", "timestamp", "timestamptz")
}
func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if len(s) >= len(p) {
			for i := 0; i+len(p) <= len(s); i++ {
				if s[i:i+len(p)] == p {
					return true
				}
			}
		}
	}
	return false
}
