-- Create signals table (configuration/metadata)
CREATE TABLE IF NOT EXISTS signals (
    id SERIAL PRIMARY KEY,
    device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    signal_type VARCHAR(50) NOT NULL DEFAULT 'analogic' CHECK (signal_type IN ('digital', 'analogic')),
    direction VARCHAR(50) NOT NULL DEFAULT 'input' CHECK (direction IN ('input', 'output')),
    sensor_name VARCHAR(255),
    description TEXT,
    unit VARCHAR(50),
    min_value DOUBLE PRECISION,
    max_value DOUBLE PRECISION,
    metadata JSONB,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create signal_values table (actual data points)
CREATE TABLE IF NOT EXISTS signal_values (
    id SERIAL PRIMARY KEY,
    signal_id INTEGER NOT NULL REFERENCES signals(id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    value DOUBLE PRECISION,
    digital_value BOOLEAN,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Drop old signals table if it exists (from previous migration)
DROP TABLE IF EXISTS signals_old CASCADE;

-- Rename old signals table if it exists
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'signals' AND table_schema = 'public') THEN
        -- Check if it's the old structure (has device_id but no name)
        IF EXISTS (
            SELECT 1 FROM information_schema.columns 
            WHERE table_name = 'signals' 
            AND column_name = 'device_id'
            AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'signals' AND column_name = 'name')
        ) THEN
            ALTER TABLE signals RENAME TO signals_old;
        END IF;
    END IF;
END $$;

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_signals_device_id ON signals(device_id);
CREATE INDEX IF NOT EXISTS idx_signals_type ON signals(signal_type);
CREATE INDEX IF NOT EXISTS idx_signals_direction ON signals(direction);
CREATE INDEX IF NOT EXISTS idx_signal_values_signal_id ON signal_values(signal_id);
CREATE INDEX IF NOT EXISTS idx_signal_values_user_id ON signal_values(user_id);
CREATE INDEX IF NOT EXISTS idx_signal_values_timestamp ON signal_values(timestamp);

