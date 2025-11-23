-- Add authentication fields to users
ALTER TABLE users ADD COLUMN IF NOT EXISTS email VARCHAR(255) UNIQUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT true;

-- Create devices table
CREATE TABLE IF NOT EXISTS devices (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    device_type VARCHAR(100),
    location VARCHAR(255),
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    auth_token VARCHAR(255) UNIQUE NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Rename readings to signals
ALTER TABLE readings RENAME TO signals;

-- Update signals table structure
ALTER TABLE signals DROP COLUMN IF EXISTS userid;
ALTER TABLE signals ADD COLUMN IF NOT EXISTS device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE;
ALTER TABLE signals ADD COLUMN IF NOT EXISTS user_id INTEGER REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE signals ADD COLUMN IF NOT EXISTS signal_type VARCHAR(50) NOT NULL DEFAULT 'analogic' CHECK (signal_type IN ('digital', 'analogic'));
ALTER TABLE signals ADD COLUMN IF NOT EXISTS sensor_name VARCHAR(255);
ALTER TABLE signals ADD COLUMN IF NOT EXISTS metadata JSONB;

-- Make value nullable for digital signals (can be 0/1 or true/false)
ALTER TABLE signals ALTER COLUMN value DROP NOT NULL;

-- Keep old columns for backward compatibility (can be removed later)
-- torquevalues, asmtimes, motionwastes, setvalue can stay for now

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_signals_device_id ON signals(device_id);
CREATE INDEX IF NOT EXISTS idx_signals_user_id ON signals(user_id);
CREATE INDEX IF NOT EXISTS idx_signals_timestamp ON signals(timestamp);
CREATE INDEX IF NOT EXISTS idx_signals_type ON signals(signal_type);
CREATE INDEX IF NOT EXISTS idx_devices_user_id ON devices(user_id);
CREATE INDEX IF NOT EXISTS idx_devices_auth_token ON devices(auth_token);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

