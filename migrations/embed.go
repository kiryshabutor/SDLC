// Package migrations встраивает SQL-миграции goose в исполняемый файл,
// чтобы схема создавалась при запуске API-сервера (REQ-INST-003).
package migrations

import "embed"

// FS — файлы миграций.
//
//go:embed *.sql
var FS embed.FS
