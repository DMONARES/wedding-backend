package handlers

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type GuestRequest struct {
	MainGuest  string   `json:"main_guest" binding:"required"`
	Comment    string   `json:"comment"`
	Companions []string `json:"companions"`
}

func AddGuestHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req GuestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("Ошибка парсинга гостя: %v\n", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		log.Printf("Guest to save: %+v\n", req)

		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
			return
		}

		var groupID int64
		err = tx.QueryRow(
			"INSERT INTO guest_groups (main_guest_name, comment, guest_count) VALUES ($1, $2, $3) RETURNING id",
			req.MainGuest,
			req.Comment,
			1+len(req.Companions),
		).Scan(&groupID)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert guest group"})
			return
		}

		_, err = tx.Exec(
			"INSERT INTO guests (group_id, name, is_main) VALUES ($1, $2, $3)",
			groupID, req.MainGuest, true,
		)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert main guest"})
			return
		}

		for _, companion := range req.Companions {
			_, err := tx.Exec(
				"INSERT INTO guests (group_id, name, is_main) VALUES ($1, $2, $3)",
				groupID, companion, false,
			)
			if err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert companion"})
				return
			}
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Guest group added successfully", "guest_group_id": groupID})
	}
}

func GetGuestsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(`
			SELECT
				gg.id AS group_id,
				gg.main_guest_name,
				gg.comment,
				g.name,
				g.is_main
			FROM guest_groups gg
			JOIN guests g ON gg.id = g.group_id
			ORDER BY gg.id, g.is_main DESC, g.id
		`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query guests"})
			return
		}
		defer rows.Close()

		type Group struct {
			GroupID    int64    `json:"group_id"`
			MainGuest  string   `json:"main_guest"`
			Comment    string   `json:"comment"`
			Companions []string `json:"companions"`
		}

		var groups []Group
		var currentGroupID int64 = -1
		var currentGroup *Group

		for rows.Next() {
			var groupID int64
			var mainGuestName, comment, guestName string
			var isMain bool

			err := rows.Scan(&groupID, &mainGuestName, &comment, &guestName, &isMain)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan guest row"})
				return
			}

			if groupID != currentGroupID {
				currentGroupID = groupID
				currentGroup = &Group{
					GroupID:    groupID,
					MainGuest:  mainGuestName,
					Comment:    comment,
					Companions: []string{},
				}
				groups = append(groups, *currentGroup)
			}

			if !isMain && currentGroup != nil {
				currentGroup.Companions = append(currentGroup.Companions, guestName)
			}
		}

		c.JSON(http.StatusOK, groups)
	}
}

func DeleteGuestGroupHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		guestID := c.Param("id")

		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
			return
		}

		// Узнаём группу и статус основного гостя у удаляемого гостя
		var groupID int64
		var isMain bool
		err = tx.QueryRow("SELECT group_id, is_main FROM guests WHERE id = $1", guestID).Scan(&groupID, &isMain)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusNotFound, gin.H{"error": "Guest not found"})
			return
		}

		// Удаляем гостя
		_, err = tx.Exec("DELETE FROM guests WHERE id = $1", guestID)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete guest"})
			return
		}

		// Если удалённый гость был основной персоной группы
		if isMain {
			// Назначаем новым основным гостем первого оставшегося гостя из группы
			var newMainGuestID int64
			var newMainGuestName string
			err = tx.QueryRow("SELECT id, name FROM guests WHERE group_id = $1 ORDER BY id LIMIT 1", groupID).Scan(&newMainGuestID, &newMainGuestName)
			if err == nil {
				// Обновляем is_main у нового гостя
				_, err = tx.Exec("UPDATE guests SET is_main = TRUE WHERE id = $1", newMainGuestID)
				if err != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update new main guest"})
					return
				}

				// Обновляем main_guest_name в guest_groups
				_, err = tx.Exec("UPDATE guest_groups SET main_guest_name = $1 WHERE id = $2", newMainGuestName, groupID)
				if err != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update guest group main guest"})
					return
				}
			} else {
				// В группе больше нет гостей — удаляем группу
				_, err = tx.Exec("DELETE FROM guest_groups WHERE id = $1", groupID)
				if err != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete empty guest group"})
					return
				}
			}
		}

		// Обновляем guest_count в guest_groups (уменьшаем на 1)
		_, err = tx.Exec("UPDATE guest_groups SET guest_count = guest_count - 1 WHERE id = $1", groupID)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update guest count"})
			return
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Guest deleted successfully"})
	}
}

type EditGuestRequest struct {
	Name string `json:"name" binding:"required"`
}

func EditGuestHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		guestID := c.Param("id")

		var req EditGuestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
			return
		}

		// Проверяем, гость ли основной
		var isMain bool
		var groupID int64
		err = tx.QueryRow("SELECT is_main, group_id FROM guests WHERE id = $1", guestID).Scan(&isMain, &groupID)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusNotFound, gin.H{"error": "Guest not found"})
			return
		}

		// Обновляем имя гостя в таблице guests
		_, err = tx.Exec("UPDATE guests SET name = $1 WHERE id = $2", req.Name, guestID)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update guest"})
			return
		}

		// Если это основной гость — обновляем main_guest_name в guest_groups
		if isMain {
			_, err = tx.Exec("UPDATE guest_groups SET main_guest_name = $1 WHERE id = $2", req.Name, groupID)
			if err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update guest group main guest"})
				return
			}
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Guest updated successfully"})
	}
}
