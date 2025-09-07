package graph

import (
	"time"

	"transaction-service/internal/db"
	"transaction-service/internal/models"
	"github.com/graphql-go/graphql"
)

type Resolver struct {
	Repo *db.TransactionRepo
}

func ParseISO(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, s)
}

func NewSchema(res *Resolver) (graphql.Schema, error) {
	// GraphQL type for a transaction. We resolve time fields as RFC3339 strings.
	transactionType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Transaction",
		Fields: graphql.Fields{
			"id":           &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"coinid":       &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"userid":       &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"dataid":       &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"coinused":     &graphql.Field{Type: graphql.NewNonNull(graphql.Float)},
			"platformName": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},

			"transactionTimestamp": &graphql.Field{
				Type: graphql.NewNonNull(graphql.String),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					switch t := p.Source.(type) {
					case models.Transaction:
						return t.TransactionTimestamp.UTC().Format(time.RFC3339), nil
					case *models.Transaction:
						return t.TransactionTimestamp.UTC().Format(time.RFC3339), nil
					default:
						return nil, nil
					}
				},
			},
			"expiryDate": &graphql.Field{
				Type: graphql.NewNonNull(graphql.String),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					switch t := p.Source.(type) {
					case models.Transaction:
						return t.ExpiryDate.UTC().Format(time.RFC3339), nil
					case *models.Transaction:
						return t.ExpiryDate.UTC().Format(time.RFC3339), nil
					default:
						return nil, nil
					}
				},
			},
		},
	})

	filterInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "TransactionFilter",
		Fields: graphql.InputObjectConfigFieldMap{
			"id":            &graphql.InputObjectFieldConfig{Type: graphql.String},
			"userid":        &graphql.InputObjectFieldConfig{Type: graphql.String},
			"coinid":        &graphql.InputObjectFieldConfig{Type: graphql.String},
			"dataid":        &graphql.InputObjectFieldConfig{Type: graphql.String},
			"platformName":  &graphql.InputObjectFieldConfig{Type: graphql.String},
			"fromTimestamp": &graphql.InputObjectFieldConfig{Type: graphql.String}, // RFC3339
			"toTimestamp":   &graphql.InputObjectFieldConfig{Type: graphql.String}, // RFC3339
			"limit":         &graphql.InputObjectFieldConfig{Type: graphql.Int},
			"offset":        &graphql.InputObjectFieldConfig{Type: graphql.Int},
		},
	})

	addInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "AddTransactionInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"coinid":               &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"userid":               &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"dataid":               &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"coinused":             &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.Float)},
			"transactionTimestamp": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)}, // RFC3339
			"expiryDate":           &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)}, // RFC3339
			"platformName":         &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		},
	})

	rootQuery := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"getTransactionByID": &graphql.Field{
				Type: transactionType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					id := p.Args["id"].(string)
					return res.Repo.GetByID(p.Context, id)
				},
			},
			"getTransactions": &graphql.Field{
				Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(transactionType))),
				Args: graphql.FieldConfigArgument{
					"filter": &graphql.ArgumentConfig{Type: filterInput},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					var f db.TransactionFilter
					if raw, ok := p.Args["filter"].(map[string]any); ok {
						if v, ok := raw["id"].(string); ok {
							f.ID = &v
						}
						if v, ok := raw["userid"].(string); ok {
							f.UserID = &v
						}
						if v, ok := raw["coinid"].(string); ok {
							f.CoinID = &v
						}
						if v, ok := raw["dataid"].(string); ok {
							f.DataID = &v
						}
						if v, ok := raw["platformName"].(string); ok {
							f.PlatformName = &v
						}
						if v, ok := raw["fromTimestamp"].(string); ok && v != "" {
							if t, err := ParseISO(v); err == nil {
								f.FromTimestamp = &t
							}
						}
						if v, ok := raw["toTimestamp"].(string); ok && v != "" {
							if t, err := ParseISO(v); err == nil {
								f.ToTimestamp = &t
							}
						}
						if v, ok := raw["limit"].(int); ok {
							f.Limit = v
						}
						if v, ok := raw["offset"].(int); ok {
							f.Offset = v
						}
					}
					return res.Repo.List(p.Context, f)
				},
			},
		},
	})

	rootMutation := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"addTransaction": &graphql.Field{
				Type: transactionType,
				Args: graphql.FieldConfigArgument{
					"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(addInput)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					in := p.Args["input"].(map[string]any)

					txTime, err := ParseISO(in["transactionTimestamp"].(string))
					if err != nil {
						return nil, err
					}
					exp, err := ParseISO(in["expiryDate"].(string))
					if err != nil {
						return nil, err
					}

					model := models.Transaction{
						CoinID:               in["coinid"].(string),
						UserID:               in["userid"].(string),
						DataID:               in["dataid"].(string),
						CoinUsed:             in["coinused"].(float64),
						TransactionTimestamp: txTime,
						ExpiryDate:           exp,
						PlatformName:         in["platformName"].(string),
					}
					return res.Repo.Insert(p.Context, model)
				},
			},
		},
	})

	return graphql.NewSchema(graphql.SchemaConfig{
		Query:    rootQuery,
		Mutation: rootMutation,
	})
}
