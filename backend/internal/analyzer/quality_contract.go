package analyzer

import "github.com/osmendes/db-auditor/internal/dqcontract"

const QualityContractViolationType = dqcontract.FindingType

func QualityContractRule() RuleMeta {
	return RuleMeta{ID: dqcontract.FindingType, Version: dqcontract.RuleVersion, Category: dqcontract.Category, Title: "Violação de contrato de qualidade", Description: "Uma checagem declarada no contrato versionado foi violada na amostra ou no agregado. Só contagens são persistidas."}
}

type RuleMeta struct{ ID, Version, Category, Title, Description string }
