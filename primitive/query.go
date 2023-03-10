package primitive

import (
	"context"
	"errors"
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/turbot/steampipe-pipelines/pipeline"
)

type Query struct{}

func (e *Query) ValidateInput(ctx context.Context, i pipeline.StepInput) error {
	if i["sql"] == nil {
		return errors.New("Query input must define sql")
	}
	return nil
}

func (e *Query) Run(ctx context.Context, input pipeline.StepInput) (pipeline.StepOutput, error) {
	if err := e.ValidateInput(ctx, input); err != nil {
		return nil, err
	}

	db, err := sqlx.Connect("postgres", "postgres://steampipe@localhost:9193/steampipe")
	if err != nil {
		log.Fatal("Failed to open a DB connection: ", err)
	}
	defer db.Close()

	sql := input["sql"].(string)

	results := []map[string]interface{}{}

	rows, err := db.Queryx(sql)
	if err != nil {
		panic(err)
	}
	for rows.Next() {
		row := make(map[string]interface{})
		err = rows.MapScan(row)
		if err != nil {
			panic(err)
		}
		results = append(results, row)
	}

	if err = rows.Err(); err != nil {
		panic(err)
	}

	output := pipeline.StepOutput{
		"rows": results,
	}

	return output, nil
}
