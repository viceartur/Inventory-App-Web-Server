package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"
)

type IncomingMaterialJSON struct {
	CustomerID   string `json:"customerId"`
	StockID      string `json:"stockId"`
	MaterialType string `json:"type"`
	Qty          string `json:"quantity"`
	Cost         string `json:"cost"`
	MinQty       string `json:"minQuantity"`
	MaxQty       string `json:"maxQuantity"`
	Description  string `json:"description"`
	Owner        string `json:"owner"`
	IsActive     bool   `json:"isActive"`
}

type IncomingMaterialDB struct {
	ShippingID   string  `field:"shipping_id"`
	CustomerName string  `field:"customer_name"`
	CustomerID   int     `field:"customer_id"`
	StockID      string  `field:"stock_id"`
	Cost         float64 `field:"cost"`
	Quantity     int     `field:"quantity"`
	MinQty       int     `field:"min_required_quantity"`
	MaxQty       int     `field:"max_required_quantity"`
	Description  string  `field:"description"`
	IsActive     bool    `field:"is_active"`
	MaterialType string  `field:"material_type"`
	Owner        string  `field:"owner"`
}

// Create Material
// Move Material
type MaterialJSON struct {
	MaterialID string `json:"materialId"`
	LocationID string `json:"locationId"`
	Qty        string `json:"quantity"`
	Notes      string `json:"notes"`
	IsPrimary  string `json:"isPrimary"`
}

// Remove Material
type MaterialToRemoveJSON struct {
	MaterialID string `json:"materialId"`
	Qty        string `json:"quantity"`
	JobTicket  string `json:"jobTicket"`
}

type MaterialDB struct {
	MaterialID    int       `field:"material_id"`
	WarehouseName string    `field:"warehouse_name"`
	StockID       string    `field:"stock_id"`
	CustomerID    int       `field:"customer_id"`
	CustomerName  string    `field:"customer_name"`
	LocationID    int       `field:"location_id"`
	LocationName  string    `field:"location_name"`
	MaterialType  string    `field:"material_type"`
	Description   string    `field:"description"`
	Notes         string    `field:"notes"`
	Quantity      int       `field:"quantity"`
	UpdatedAt     time.Time `field:"updated_at"`
	IsActive      bool      `field:"is_active"`
	MinQty        int       `field:"min_required_quantity"`
	MaxQty        int       `field:"max_required_quantity"`
	Owner         string    `field:"onwer"`
	IsPrimary     bool      `field:"is_primary"`
}

type MaterialFilter struct {
	stockId      string
	customerName string
	description  string
	locationName string
}

type Price struct {
	priceId    int
	materialId int
	qty        int
	cost       float64
}

type PriceToRemove struct {
	materialId int
	qty        int
	notes      string
	jobTicket  string
}

type PriceDB struct {
	PriceID    int     `field:"price_id"`
	MaterialID int     `field:"material_id"`
	Qty        int     `field:"quantity"`
	Cost       float64 `field:"cost"`
}

type TransactionInfo struct {
	priceId   int       `field:"price_id"`
	qty       int       `field:"quantity_change"`
	notes     string    `field:"notes"`
	jobTicket string    `field:"job_ticket"`
	updatedAt time.Time `field:"updated_at"`
}

func fetchMaterialTypes(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
		SELECT enumlabel FROM pg_enum pe
		LEFT JOIN pg_type pt ON pt.oid = pe.enumtypid
		WHERE pt.typname = 'material_type';
	`)
	if err != nil {
		return []string{}, err
	}

	var materialTypes []string
	for rows.Next() {
		var materialType string
		if err := rows.Scan(&materialType); err != nil {
			return nil, fmt.Errorf("Error scanning row: %w", err)
		}
		materialTypes = append(materialTypes, materialType)
	}

	return materialTypes, nil
}

func sendMaterial(material IncomingMaterialJSON, db *sql.DB) error {
	qty, _ := strconv.Atoi(material.Qty)
	minQty, _ := strconv.Atoi(material.MinQty)
	maxQty, _ := strconv.Atoi(material.MaxQty)

	_, err := db.Query(`
				INSERT INTO incoming_materials
					(customer_id, stock_id, cost, quantity,
					max_required_quantity, min_required_quantity,
					description, is_active, type, owner)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		material.CustomerID, material.StockID, material.Cost,
		qty, maxQty, minQty,
		material.Description, material.IsActive, material.MaterialType,
		material.Owner,
	)

	if err != nil {
		return err
	}
	return nil
}

func getIncomingMaterials(db *sql.DB) ([]IncomingMaterialDB, error) {
	rows, err := db.Query(`
		SELECT shipping_id, c.name, c.customer_id, stock_id, cost, quantity,
		min_required_quantity, max_required_quantity, description, is_active, type, owner
		FROM incoming_materials im
		LEFT JOIN customers c ON c.customer_id = im.customer_id
		`)
	if err != nil {
		return nil, fmt.Errorf("Error querying incoming materials: %w", err)
	}
	defer rows.Close()

	var materials []IncomingMaterialDB
	for rows.Next() {
		var material IncomingMaterialDB
		if err := rows.Scan(
			&material.ShippingID,
			&material.CustomerName,
			&material.CustomerID,
			&material.StockID,
			&material.Cost,
			&material.Quantity,
			&material.MinQty,
			&material.MaxQty,
			&material.Description,
			&material.IsActive,
			&material.MaterialType,
			&material.Owner,
		); err != nil {
			return nil, fmt.Errorf("Error scanning row: %w", err)
		}
		materials = append(materials, material)
	}
	return materials, nil
}

func getMaterials(db *sql.DB, opts *MaterialFilter) ([]MaterialDB, error) {
	rows, err := db.Query(`
		SELECT material_id,
		COALESCE(w.name,'None') as "warehouse_name",
		c.name as "customer_name", c.customer_id,
		COALESCE(l.location_id, 0) as "location_id",
		COALESCE(l.name, 'None') as "location_name",
		stock_id, quantity, min_required_quantity, max_required_quantity,
		m.description, COALESCE(notes,'None') as "notes",
		is_active, material_type, owner,
		COALESCE(is_primary,false) as "is_primary"
		FROM materials m
		LEFT JOIN customers c ON c.customer_id = m.customer_id
		LEFT JOIN locations l ON l.location_id = m.location_id
		LEFT JOIN warehouses w ON w.warehouse_id = l.warehouse_id
		WHERE
			m.location_id IS NOT NULL AND
			($1 = '' OR m.stock_id ILIKE '%' || $1 || '%') AND
			($2 = '' OR c.name ILIKE '%' || $2 || '%') AND
			($3 = '' OR m.description ILIKE '%' || $3 || '%') AND
			($4 = '' OR l.name ILIKE '%' || $4 || '%')
		ORDER BY m.stock_id ASC;
		`, opts.stockId, opts.customerName, opts.description, opts.locationName)
	if err != nil {
		return nil, fmt.Errorf("Error querying incoming materials: %w", err)
	}
	defer rows.Close()

	var materials []MaterialDB
	for rows.Next() {
		var material MaterialDB
		if err := rows.Scan(
			&material.MaterialID,
			&material.WarehouseName,
			&material.CustomerName,
			&material.CustomerID,
			&material.LocationID,
			&material.LocationName,
			&material.StockID,
			&material.Quantity,
			&material.MinQty,
			&material.MaxQty,
			&material.Description,
			&material.Notes,
			&material.IsActive,
			&material.MaterialType,
			&material.Owner,
			&material.IsPrimary,
		); err != nil {
			return nil, fmt.Errorf("Error scanning row: %w", err)
		}
		materials = append(materials, material)
	}
	return materials, nil
}

func getMaterialPrices(tx *sql.Tx, materialId int) (map[int]Price, error) {
	rows, err := tx.Query(`
		SELECT * FROM prices
		WHERE material_id = $1
		AND quantity > 0
		ORDER BY price_id ASC;
	`, materialId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prices := make(map[int]Price)

	for rows.Next() {
		var price PriceDB
		err := rows.Scan(&price.PriceID, &price.MaterialID, &price.Qty, &price.Cost)
		if err != nil {
			return nil, err
		}

		prices[price.PriceID] = Price{qty: price.Qty, cost: price.Cost}
	}
	return prices, nil
}

// The internal method changes the Prices quantity and returns updated Prices
func updatePriceQty(tx *sql.Tx, priceInfo *Price) (float64, error) {
	var updatedCost float64
	err := tx.QueryRow(`
					UPDATE prices
					SET quantity = (quantity + $2)
					WHERE price_id = $1
					RETURNING cost;
					`, priceInfo.priceId, priceInfo.qty,
	).Scan(&updatedCost)
	if err != nil {
		return 0, err
	}

	return updatedCost, nil
}

func upsertPrice(tx *sql.Tx, priceInfo Price) (int, error) {
	var priceId int
	rows, err := tx.Query(`
					INSERT INTO prices (material_id, quantity, cost)
						VALUES ($1, $2, $3)
					ON CONFLICT (material_id, cost)
						DO UPDATE
							SET quantity = (prices.quantity + EXCLUDED.quantity)
					RETURNING price_id;
					`, priceInfo.materialId, priceInfo.qty, priceInfo.cost,
	)
	if err != nil {
		return 0, err
	}

	for rows.Next() {
		err := rows.Scan(&priceId)
		if err != nil {
			return 0, err
		}
	}
	return priceId, nil
}

func addTranscation(trx *TransactionInfo, tx *sql.Tx) error {
	rows, err := tx.Query(`
			INSERT INTO transactions_log (price_id, quantity_change, notes, job_ticket, updated_at)
			VALUES($1, $2, $3, $4, $5);
		`, trx.priceId, trx.qty, trx.notes, trx.jobTicket, trx.updatedAt)
	if err != nil {
		return err
	}
	rows.Close()
	return nil
}

func removePricesFIFO(tx *sql.Tx, priceToRemove PriceToRemove) ([]Price, error) {
	materialId := priceToRemove.materialId
	qty := priceToRemove.qty
	notes := priceToRemove.notes
	jobTicket := priceToRemove.jobTicket

	materialPrices, err := getMaterialPrices(tx, materialId)
	if err != nil {
		return nil, err
	}

	removedPrices := []Price{}

	remainingQty := qty
	for priceId, priceInfo := range materialPrices {
		if remainingQty <= priceInfo.qty {
			qtyToRemove := remainingQty
			priceInfo := &Price{
				priceId: priceId,
				qty:     -qtyToRemove,
			}

			cost, err := updatePriceQty(tx, priceInfo)
			if err != nil {
				return nil, err
			}

			err = addTranscation(&TransactionInfo{
				priceId:   priceId,
				qty:       -qtyToRemove,
				notes:     notes,
				jobTicket: jobTicket,
				updatedAt: time.Now(),
			}, tx)
			if err != nil {
				return nil, err
			}

			remainingQty = 0
			removedPrices = append(removedPrices, Price{qty: qtyToRemove, cost: cost})
			break
		} else {
			qtyToRemove := priceInfo.qty
			priceInfo := &Price{
				priceId: priceId,
				qty:     -qtyToRemove,
			}

			cost, err := updatePriceQty(tx, priceInfo)
			if err != nil {
				return nil, err
			}

			err = addTranscation(&TransactionInfo{
				priceId:   priceId,
				qty:       -qtyToRemove,
				notes:     notes,
				jobTicket: jobTicket,
				updatedAt: time.Now(),
			}, tx)
			if err != nil {
				return nil, err
			}

			remainingQty -= qtyToRemove
			removedPrices = append(removedPrices, Price{qty: qtyToRemove, cost: cost})
		}
	}
	return removedPrices, nil
}

// The method creates/updates a Material, its Prices, adds a Transaction Log, and deletes the Material from Incoming.
// Method's Context: Material Creation. The Transaction Rollback is executed once an error occurs.
func createMaterial(ctx context.Context, db *sql.DB, material MaterialJSON) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Commit() // commit only if the method is done

	var incomingMaterial IncomingMaterialDB
	err = tx.QueryRow(`
		SELECT customer_id, stock_id, cost, min_required_quantity,
		max_required_quantity, description, is_active, type, owner
		FROM incoming_materials
		WHERE shipping_id = $1`, material.MaterialID).
		Scan(
			&incomingMaterial.CustomerID,
			&incomingMaterial.StockID,
			&incomingMaterial.Cost,
			&incomingMaterial.MinQty,
			&incomingMaterial.MaxQty,
			&incomingMaterial.Description,
			&incomingMaterial.IsActive,
			&incomingMaterial.MaterialType,
			&incomingMaterial.Owner,
		)
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	// Update material in the current location if location exists
	var materialId int
	rows, err := tx.Query(`
					UPDATE materials
					SET quantity = (quantity + $1),
						notes = $2
					WHERE stock_id = $3
						AND location_id = $4
						AND owner = $5
					RETURNING material_id;
					`, material.Qty, material.Notes, incomingMaterial.StockID, material.LocationID, incomingMaterial.Owner,
	)
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	for rows.Next() {
		err := rows.Scan(&materialId)
		if err != nil {
			tx.Rollback()
			return 0, err
		}
	}
	rows.Close()

	// Upsert Prices
	var priceId int

	if materialId != 0 {
		qty, _ := strconv.Atoi(material.Qty)
		priceInfo := Price{materialId: materialId, qty: qty, cost: incomingMaterial.Cost}
		priceId, err = upsertPrice(tx, priceInfo)
		if err != nil {
			tx.Rollback()
			return 0, err
		}
	} else {
		// If there is no a Material in the chosen Location:
		// 1. Check for a NULL location and if it exists then assign the new location and qty
		rows, err := tx.Query(`
			SELECT material_id as material_id FROM materials
			WHERE location_id is NULL
				AND stock_id = $1
				AND owner = $2;
		`, incomingMaterial.StockID, incomingMaterial.Owner)
		if err != nil {
			tx.Rollback()
			return 0, err
		}

		for rows.Next() {
			err := rows.Scan(&materialId)
			if err != nil {
				tx.Rollback()
				return 0, err
			}
		}

		if materialId != 0 {
			rows, err := tx.Query(`
					UPDATE materials
					SET quantity = $1,
						notes = $2,
						location_id = $3
					WHERE material_id = $4;
					`, material.Qty, material.Notes, material.LocationID, materialId,
			)
			if err != nil {
				tx.Rollback()
				return 0, err
			}
			rows.Close()
		} else {
			// 2. If there is no a NULL Location, then add the material to the new location
			err := tx.QueryRow(`
						INSERT INTO materials
						(
							stock_id,
							location_id,
							customer_id,
							material_type,
							description,
							notes,
							quantity,
							updated_at,
							min_required_quantity,
							max_required_quantity,
							is_active,
							owner
						)
						VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING material_id;`,
				incomingMaterial.StockID,
				material.LocationID,
				incomingMaterial.CustomerID,
				incomingMaterial.MaterialType,
				incomingMaterial.Description,
				material.Notes,
				material.Qty,
				time.Now(),
				incomingMaterial.MinQty,
				incomingMaterial.MaxQty,
				incomingMaterial.IsActive,
				incomingMaterial.Owner,
			).Scan(&materialId)
			if err != nil {
				tx.Rollback()
				return 0, err
			}
		}

		// Upsert Prices
		qty, _ := strconv.Atoi(material.Qty)
		priceInfo := Price{materialId: materialId, qty: qty, cost: incomingMaterial.Cost}
		priceId, err = upsertPrice(tx, priceInfo)
		if err != nil {
			tx.Rollback()
			return 0, err
		}
	}

	// Remove the Material from Incoming
	shippingId, _ := strconv.Atoi(material.MaterialID)
	err = deleteIncomingMaterial(tx, shippingId)
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	// Add a Transaction
	qty, _ := strconv.Atoi(material.Qty)
	trxInfo := &TransactionInfo{
		priceId:   priceId,
		qty:       qty,
		notes:     material.Notes,
		updatedAt: time.Now(),
	}
	err = addTranscation(trxInfo, tx)
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	return materialId, nil
}

func deleteIncomingMaterial(tx *sql.Tx, shippingId int) error {
	if _, err := tx.Exec(`
			DELETE FROM incoming_materials WHERE shipping_id = $1;`,
		shippingId); err != nil {
		return err
	}

	return nil
}

func getMaterialById(materialId int, tx *sql.Tx) (MaterialDB, error) {
	var currMaterial MaterialDB
	err := tx.QueryRow(`SELECT * FROM materials WHERE material_id = $1`, materialId).
		Scan(
			&currMaterial.MaterialID,
			&currMaterial.StockID,
			&currMaterial.LocationID,
			&currMaterial.CustomerID,
			&currMaterial.MaterialType,
			&currMaterial.Description,
			&currMaterial.Notes,
			&currMaterial.Quantity,
			&currMaterial.MinQty,
			&currMaterial.MaxQty,
			&currMaterial.UpdatedAt,
			&currMaterial.IsActive,
			&currMaterial.Owner,
		)
	if err != nil {
		return MaterialDB{}, err
	}

	return currMaterial, nil
}

// The method changes the Material quantity at the current and new Location, its Prices, and adds Transaction Logs.
// Method's Context: Material Moving. The Transaction Rollback is executed once an error occurs.
func moveMaterial(ctx context.Context, db *sql.DB, material MaterialJSON) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Commit() // commit only if the method is done

	materialId, _ := strconv.Atoi(material.MaterialID)
	currMaterial, err := getMaterialById(materialId, tx)
	if err != nil {
		return err
	}

	newLocationId := material.LocationID
	quantity, _ := strconv.Atoi(material.Qty)
	actualQuantity := currMaterial.Quantity
	currMaterialId := currMaterial.MaterialID
	currentLocationId := currMaterial.LocationID
	stockId := currMaterial.StockID
	owner := currMaterial.Owner
	currNotes := currMaterial.Notes

	// 1. Update the Material in the current Location

	// Check whether remaining quantity exists
	if actualQuantity < quantity {
		return errors.New(
			`The moving quantity (` +
				strconv.Itoa(quantity) + `) is more than the actual one (` +
				strconv.Itoa(actualQuantity) + `)`)
	} else if actualQuantity > quantity {
		// Update material in the current location
		err = tx.QueryRow(`
			UPDATE materials
			SET quantity = (quantity - $1),
				notes = $2
			WHERE material_id = $3 AND location_id = $4
			RETURNING material_id, stock_id, location_id, customer_id, material_type,
					description, notes, quantity, updated_at, is_active,
					min_required_quantity, max_required_quantity, owner;
			`, quantity, currNotes, currMaterialId, currentLocationId,
		).Scan(
			&currMaterial.MaterialID,
			&currMaterial.StockID,
			&currMaterial.LocationID,
			&currMaterial.CustomerID,
			&currMaterial.MaterialType,
			&currMaterial.Description,
			&currMaterial.Notes,
			&currMaterial.Quantity,
			&currMaterial.UpdatedAt,
			&currMaterial.IsActive,
			&currMaterial.MinQty,
			&currMaterial.MaxQty,
			&currMaterial.Owner,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	} else if actualQuantity == quantity {
		defer func() error {
			_, err = tx.Exec(`
				UPDATE materials
				SET location_id = NULL,
					quantity = 0
				WHERE material_id = $1`,
				currMaterialId,
			)
			if err != nil {
				tx.Rollback()
				return err
			}
			return nil
		}()
	}

	// 1.1. Update Prices for the current Location

	// Remove Prices for the current Material ID
	priceToRemove := PriceToRemove{
		materialId: currMaterialId,
		qty:        quantity,
		notes:      "Moved TO a Location",
		jobTicket:  "Auto-Ticket: " + time.Now().Local().String(),
	}
	removedPrices, err := removePricesFIFO(tx, priceToRemove)
	if err != nil {
		tx.Rollback()
		return err
	}

	// 2. Update a Material in the new Location
	var newMaterialId int

	// Find an existing Material in the Location
	rows, err := db.Query(`
			UPDATE materials
			SET quantity = (quantity + $1)
			WHERE
				stock_id = $2 AND
				location_id = $3 AND
				owner = $4
			RETURNING material_id;
				`, quantity, stockId, newLocationId, owner,
	)
	if err != nil {
		return err
	}
	for rows.Next() {
		err := rows.Scan(&newMaterialId)
		if err != nil {
			return err
		}
	}

	// If there is no a Material in the new Location, then create it
	if newMaterialId == 0 {
		err = db.QueryRow(`
				INSERT INTO materials
					(stock_id, location_id,
					customer_id, material_type, description, notes, quantity, updated_at,
					is_active, min_required_quantity, max_required_quantity, owner)
					VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
					RETURNING material_id;`,
			stockId, newLocationId,
			currMaterial.CustomerID, currMaterial.MaterialType, currMaterial.Description,
			currNotes, quantity, time.Now(), currMaterial.IsActive,
			currMaterial.MinQty, currMaterial.MaxQty, currMaterial.Owner).
			Scan(&newMaterialId)
		if err != nil {
			return err
		}
	}

	// 2.2. Update Prices for the new Location and Material ID

	for i := 0; i < len(removedPrices); i++ {
		qty := removedPrices[i].qty
		cost := removedPrices[i].cost
		priceInfo := Price{materialId: newMaterialId, qty: qty, cost: cost}

		priceId, err := upsertPrice(tx, priceInfo)
		if err != nil {
			tx.Rollback()
			return err
		}

		err = addTranscation(&TransactionInfo{
			priceId:   priceId,
			qty:       qty,
			notes:     "Moved FROM a Location",
			jobTicket: "Auto-Ticket: " + time.Now().Local().String(),
			updatedAt: time.Now(),
		}, tx)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return nil
}

// The method removes a specific Material quantity, its Prices, adds a Transaction Log.
// Method's Context: Material Removing. The Transaction Rollback is executed once an error occurs.
func removeMaterial(ctx context.Context, db *sql.DB, material MaterialToRemoveJSON) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Commit() // commit only if the method is done

	materialId, _ := strconv.Atoi(material.MaterialID)
	currMaterial, err := getMaterialById(materialId, tx)
	if err != nil {
		tx.Rollback()
		return errors.New("Unable to get the current material info: " + err.Error())
	}

	quantity, _ := strconv.Atoi(material.Qty)
	actualQuantity := currMaterial.Quantity
	jobTicket := material.JobTicket

	if actualQuantity < quantity {
		return errors.New(`The removing quantity (` + strconv.Itoa(quantity) + `) is more than the actual one (` + strconv.Itoa(actualQuantity) + `)`)
	} else if actualQuantity == quantity {
		_, err = tx.Exec(`
			UPDATE materials
			SET location_id = NULL,
				quantity = 0
			WHERE material_id = $1
		`, materialId)
		if err != nil {
			tx.Rollback()
			return err
		}
	} else {
		// Update the material quantity
		_, err = tx.Exec(`
				UPDATE materials
				SET quantity = (quantity - $1)
				WHERE material_id = $2;
		`, quantity, materialId,
		)
	}
	if err != nil {
		tx.Rollback()
		return err
	}

	priceToRemove := PriceToRemove{
		materialId: materialId,
		qty:        quantity,
		notes:      "Removed FROM a Location",
		jobTicket:  jobTicket,
	}
	_, err = removePricesFIFO(tx, priceToRemove)
	if err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

func updateMaterial(db *sql.DB, material MaterialJSON) error {
	materialId, _ := strconv.Atoi(material.MaterialID)
	_, err := db.Exec(`
		UPDATE materials
		SET is_primary = $2
		WHERE material_id = $1
	`, materialId, material.IsPrimary)
	if err != nil {
		return err
	}
	return nil
}
