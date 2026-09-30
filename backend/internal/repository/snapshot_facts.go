package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/mayconmendes-qc/db-auditor/internal/analyzer"
)

// LoadSnapshotFacts loads every analyzer input from one explicitly selected run.
func (s *Store) LoadSnapshotFacts(ctx context.Context, environmentID, auditRunID string) (analyzer.SnapshotFacts, error) {
	f := analyzer.SnapshotFacts{EnvironmentID: environmentID, AuditRunID: auditRunID}
	queries := []func() error{
		func() error {
			rows, err := s.pool.Query(ctx, `SELECT database_name,schema_name,table_name,total_size_bytes,collected_at,n_live_tup,n_dead_tup,COALESCE(last_vacuum::text,''),COALESCE(last_autovacuum::text,''),n_tup_ins,n_tup_upd,n_tup_del,stats_reset FROM table_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid`, environmentID, auditRunID)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var t analyzer.TableFact
				var v analyzer.VacuumFact
				var a analyzer.ActivityFact
				var statsReset *time.Time
				if err := rows.Scan(&t.Database, &t.Schema, &t.Name, &t.SizeBytes, &t.CollectedAt, &v.NLiveTup, &v.NDeadTup, &v.LastVacuum, &v.LastAutovacuum, &a.NTupIns, &a.NTupUpd, &a.NTupDel, &statsReset); err != nil {
					return err
				}
				v.Database, v.Schema, v.Name = t.Database, t.Schema, t.Name
				a.Database, a.Schema, a.Name, a.ObjectType, a.NLiveTup = t.Database, t.Schema, t.Name, "table", v.NLiveTup
				if statsReset != nil && a.NTupIns == 0 && a.NTupUpd == 0 && a.NTupDel == 0 {
					a.DaysSinceDML = int(t.CollectedAt.Sub(*statsReset).Hours() / 24)
					f.Activity = append(f.Activity, a)
				}
				f.Tables = append(f.Tables, t)
				f.Vacuum = append(f.Vacuum, v)
			}
			return rows.Err()
		},
		func() error {
			rows, err := s.pool.Query(ctx, `SELECT database_name,schema_name,table_name,index_name,idx_scan,size_bytes,is_primary,is_unique,index_definition,collected_at,stats_reset FROM index_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid`, environmentID, auditRunID)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var x analyzer.IndexFact
				if err := rows.Scan(&x.Database, &x.Schema, &x.TableName, &x.IndexName, &x.IdxScan, &x.SizeBytes, &x.IsPrimary, &x.IsUnique, &x.Definition, &x.CollectedAt, &x.StatsReset); err != nil {
					return err
				}
				f.Indexes = append(f.Indexes, x)
			}
			return rows.Err()
		},
		func() error {
			rows, err := s.pool.Query(ctx, `SELECT database_name,schema_name,hypertable_name,num_chunks,total_size_bytes FROM hypertable_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid`, environmentID, auditRunID)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var x analyzer.HypertableFact
				if err := rows.Scan(&x.Database, &x.Schema, &x.Name, &x.NumChunks, &x.SizeBytes); err != nil {
					return err
				}
				f.Hypertables = append(f.Hypertables, x)
			}
			return rows.Err()
		},
		func() error {
			rows, err := s.pool.Query(ctx, `SELECT database_name,schema_name,hypertable_name,chunk_name,total_size_bytes FROM chunk_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid`, environmentID, auditRunID)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var x analyzer.ChunkFact
				if err := rows.Scan(&x.Database, &x.Schema, &x.HypertableName, &x.ChunkName, &x.SizeBytes); err != nil {
					return err
				}
				f.Chunks = append(f.Chunks, x)
			}
			return rows.Err()
		},
		func() error {
			rows, err := s.pool.Query(ctx, `SELECT c.database_name,c.schema_name,c.view_name,COALESCE(c.materialization_schema,''),COALESCE(c.materialization_hypertable,''),c.materialized_only,EXISTS(SELECT 1 FROM policy_snapshot p WHERE p.audit_run_id=c.audit_run_id AND p.database_name=c.database_name AND p.policy_type='refresh' AND ((p.hypertable_schema=c.schema_name AND p.hypertable_name=c.view_name) OR (p.hypertable_schema=c.materialization_schema AND p.hypertable_name=c.materialization_hypertable))),c.view_definition FROM continuous_aggregate_snapshot c WHERE c.environment_id=$1::uuid AND c.audit_run_id=$2::uuid`, environmentID, auditRunID)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var x analyzer.CAGGFact
				if err := rows.Scan(&x.Database, &x.Schema, &x.ViewName, &x.MaterializationSchema, &x.MaterializationHypertable, &x.MaterializedOnly, &x.HasRefreshPolicy, &x.ViewDefinition); err != nil {
					return err
				}
				f.CAGGs = append(f.CAGGs, x)
			}
			return rows.Err()
		},
		func() error {
			rows, err := s.pool.Query(ctx, `SELECT p.database_name,p.job_id,p.policy_type,COALESCE(p.proc_name,''),COALESCE(p.hypertable_schema,''),COALESCE(p.hypertable_name,''),COALESCE(p.schedule_interval,''),COALESCE(p.config_json,''),p.scheduled,COALESCE(j.last_run_status,'') FROM policy_snapshot p LEFT JOIN job_snapshot j ON j.audit_run_id=p.audit_run_id AND j.database_name=p.database_name AND j.job_id=p.job_id WHERE p.environment_id=$1::uuid AND p.audit_run_id=$2::uuid`, environmentID, auditRunID)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var x analyzer.PolicyFact
				if err := rows.Scan(&x.Database, &x.JobID, &x.PolicyType, &x.ProcName, &x.HypertableSchema, &x.HypertableName, &x.ScheduleInterval, &x.Config, &x.Scheduled, &x.LastRunStatus); err != nil {
					return err
				}
				f.Policies = append(f.Policies, x)
			}
			return rows.Err()
		},
		func() error {
			rows, err := s.pool.Query(ctx, `SELECT database_name,job_id,COALESCE(application_name,''),COALESCE(proc_name,''),scheduled,COALESCE(last_run_status,''),total_failures FROM job_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid`, environmentID, auditRunID)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var x analyzer.JobFact
				if err := rows.Scan(&x.Database, &x.JobID, &x.Application, &x.ProcName, &x.Scheduled, &x.LastRunStatus, &x.TotalFailures); err != nil {
					return err
				}
				f.Jobs = append(f.Jobs, x)
			}
			return rows.Err()
		},
		func() error {
			rows, err := s.pool.Query(ctx, `SELECT database_name,schema_name,function_name,identity_arguments,is_security_definer,COALESCE(owner_name,''),COALESCE(language_name,'') FROM function_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid`, environmentID, auditRunID)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var x analyzer.FunctionSecurityFact
				if err := rows.Scan(&x.Database, &x.Schema, &x.FunctionName, &x.IdentityArgs, &x.IsSecurityDefiner, &x.Owner, &x.Language); err != nil {
					return err
				}
				f.Functions = append(f.Functions, x)
			}
			return rows.Err()
		},
	}
	for i, query := range queries {
		if err := query(); err != nil {
			return f, fmt.Errorf("snapshot fact query %d: %w", i+1, err)
		}
	}
	return f, nil
}
