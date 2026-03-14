-- Add planned_delivery_date column to production_orders table
ALTER TABLE production_orders
ADD COLUMN IF NOT EXISTS planned_delivery_date TIMESTAMP WITH TIME ZONE;