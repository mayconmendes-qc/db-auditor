-- A comparação de carga é uma confirmação explícita do operador. Ela não é
-- inferida a partir do perfil de coleta, que descreve apenas os coletores.
ALTER TABLE finding_action_measurement
  ADD COLUMN IF NOT EXISTS workload_comparable boolean NOT NULL DEFAULT false;

-- Registros antigos não tinham confirmação da equivalência de carga.
-- Preserve a medição e retire apenas a indicação de comparação confiável.
UPDATE finding_action_measurement
SET comparable = false,
    comparison_note = 'Carga de trabalho não confirmada; revise a medição antes de comparar.'
WHERE workload_comparable = false AND comparable = true;
