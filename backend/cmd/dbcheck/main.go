package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	db, err := sql.Open("pgx", os.Args[1])
	if err != nil { fmt.Println("open err", err); return }
	defer db.Close()
	rows, _ := db.Query(`SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns WHERE table_name='conservation_activities' AND column_name IN ('current_event_id','restoration_request_id','manager_id')`)
	for rows.Next() {
		var name, dtype, nullable string
		var def sql.NullString
		rows.Scan(&name, &dtype, &nullable, &def)
		fmt.Printf("%s: type=%s nullable=%s default=%v\n", name, dtype, nullable, def.String)
	}
}
