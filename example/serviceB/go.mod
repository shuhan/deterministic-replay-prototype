module serviceB

go 1.24.0

replace example.com/sdk => ../../sdk

require example.com/sdk v0.0.0-00010101000000-000000000000

require github.com/google/uuid v1.6.0 // indirect
