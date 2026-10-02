package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSprint20LargeInventoryPagingIntegration(t *testing.T) {
	dsn:=os.Getenv("AUDITOR_TEST_DATABASE_URL")
	if dsn=="" || os.Getenv("AUDITOR_TEST_DISPOSABLE")!="1" || os.Getenv("AUDITOR_TEST_SCALE")!="1" {t.Skip("scale test requires disposable database and explicit opt-in")}
	ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second)
	defer cancel()
	pool,err:=pgxpool.New(ctx,dsn);if err!=nil {t.Fatal(err)};defer pool.Close()
	var env,run string
	if err:=pool.QueryRow(ctx,`INSERT INTO audit_environment(name,type,discovery_mode) VALUES(gen_random_uuid()::text,'self_hosted','multi_database') RETURNING id::text`).Scan(&env);err!=nil {t.Fatal(err)}
	if err:=pool.QueryRow(ctx,`INSERT INTO audit_run(environment_id,profile,status,service_version,collector_version) VALUES($1::uuid,'manual','success','test','test') RETURNING id::text`,env).Scan(&run);err!=nil {t.Fatal(err)}
	_,err=pool.Exec(ctx,`INSERT INTO table_snapshot(audit_run_id,environment_id,database_name,schema_name,table_name)
		SELECT $1::uuid,$2::uuid,'db'||d,'schema'||s,'table'||l FROM generate_series(1,5)d CROSS JOIN generate_series(1,20)s CROSS JOIN generate_series(1,100)l`,run,env)
	if err!=nil {t.Fatal(err)}
	store:=NewStore(pool)
	start:=time.Now()
	items,total,err:=store.ListTableSnapshots(ctx,InventoryFilter{EnvironmentID:env,Limit:50,Offset:9950})
	if err!=nil || total!=10000 || len(items)!=50 {t.Fatalf("page %d/%d %v",len(items),total,err)}
	if elapsed:=time.Since(start);elapsed>10*time.Second {t.Errorf("large page took %v",elapsed)}
	if _,err:=pool.Exec(ctx,`ANALYZE table_snapshot`);err!=nil {t.Fatal(err)}
	rows,err:=pool.Query(ctx,`EXPLAIN (ANALYZE, BUFFERS) SELECT id FROM table_snapshot WHERE environment_id=$1::uuid AND audit_run_id=$2::uuid ORDER BY database_name,schema_name,table_name,id LIMIT 50 OFFSET 9950`,env,run)
	if err!=nil {t.Fatal(err)}
	defer rows.Close()
	for rows.Next(){var line string;if err:=rows.Scan(&line);err!=nil{t.Fatal(err)};t.Log(line)}
	if err:=rows.Err();err!=nil{t.Fatal(err)}
}
