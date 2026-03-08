# 跑全部（原本的行為）
go run cmd/etl-telemetry/main.go

# 只跑指定電號
ETL_UTILITY_IDS=05755a6b1a1 go run cmd/etl-telemetry/main.go

# 多個電號
ETL_UTILITY_IDS=05755a6b1a1,12345x go run cmd/etl-telemetry/main.go

# 指定電號 + 只跑 SE 類型
ETL_UTILITY_IDS=05755a6b1a1 ETL_DEVICE_TYPES=SE go run cmd/etl-telemetry/main.go

# 指定電號 + 時間範圍
ETL_UTILITY_IDS=0415038011a ETL_FROM=2026-01-01 ETL_TO=2026-06-30 go run cmd/etl-telemetry/main.go