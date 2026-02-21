-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Users table (riders)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    phone VARCHAR(15) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255),
    rating DECIMAL(2,1) DEFAULT 5.0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Drivers table
CREATE TABLE drivers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    phone VARCHAR(15) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255),
    license_number VARCHAR(50) NOT NULL,
    vehicle_type VARCHAR(20) NOT NULL,
    vehicle_number VARCHAR(20) NOT NULL,
    status VARCHAR(20) DEFAULT 'offline',
    rating DECIMAL(2,1) DEFAULT 5.0,
    total_trips INTEGER DEFAULT 0,
    current_lat DECIMAL(10, 8),
    current_lng DECIMAL(11, 8),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Rides table (ride requests)
CREATE TABLE rides (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    driver_id UUID REFERENCES drivers(id),

    -- Locations
    pickup_lat DECIMAL(10, 8) NOT NULL,
    pickup_lng DECIMAL(11, 8) NOT NULL,
    pickup_address TEXT,
    dropoff_lat DECIMAL(10, 8) NOT NULL,
    dropoff_lng DECIMAL(11, 8) NOT NULL,
    dropoff_address TEXT,

    -- Ride details
    vehicle_type VARCHAR(20) NOT NULL,
    status VARCHAR(30) DEFAULT 'pending',

    -- Pricing
    estimated_fare DECIMAL(10, 2),
    surge_multiplier DECIMAL(3, 2) DEFAULT 1.00,
    estimated_distance_km DECIMAL(10, 2),
    estimated_duration_mins INTEGER,

    -- Metadata
    payment_method VARCHAR(20) DEFAULT 'cash',
    idempotency_key VARCHAR(64) UNIQUE,
    cancelled_by VARCHAR(20),
    cancellation_reason TEXT,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Trips table (actual trip after ride starts)
CREATE TABLE trips (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ride_id UUID NOT NULL REFERENCES rides(id),
    driver_id UUID NOT NULL REFERENCES drivers(id),
    user_id UUID NOT NULL REFERENCES users(id),

    -- Trip tracking
    status VARCHAR(20) DEFAULT 'started',
    start_time TIMESTAMP WITH TIME ZONE,
    end_time TIMESTAMP WITH TIME ZONE,
    pause_duration_secs INTEGER DEFAULT 0,

    -- Actual metrics
    actual_distance_km DECIMAL(10, 2),
    actual_duration_mins INTEGER,
    route_polyline TEXT,

    -- Fare breakdown
    base_fare DECIMAL(10, 2),
    distance_fare DECIMAL(10, 2),
    time_fare DECIMAL(10, 2),
    surge_amount DECIMAL(10, 2),
    total_fare DECIMAL(10, 2),

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Payments table
CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    trip_id UUID NOT NULL REFERENCES trips(id),
    user_id UUID NOT NULL REFERENCES users(id),
    driver_id UUID NOT NULL REFERENCES drivers(id),

    amount DECIMAL(10, 2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'INR',
    method VARCHAR(20) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',

    -- PSP details
    psp_transaction_id VARCHAR(100),
    psp_response JSONB,

    idempotency_key VARCHAR(64) UNIQUE,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Driver ride offers (tracking which drivers were offered a ride)
CREATE TABLE ride_offers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ride_id UUID NOT NULL REFERENCES rides(id),
    driver_id UUID NOT NULL REFERENCES drivers(id),

    status VARCHAR(20) DEFAULT 'pending',
    offered_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    responded_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE,

    UNIQUE(ride_id, driver_id)
);

-- Indexes for performance
CREATE INDEX idx_users_phone ON users(phone);

CREATE INDEX idx_drivers_status ON drivers(status);
CREATE INDEX idx_drivers_vehicle_type ON drivers(vehicle_type);
CREATE INDEX idx_drivers_phone ON drivers(phone);

CREATE INDEX idx_rides_user_id ON rides(user_id);
CREATE INDEX idx_rides_driver_id ON rides(driver_id);
CREATE INDEX idx_rides_status ON rides(status);
CREATE INDEX idx_rides_created_at ON rides(created_at DESC);
CREATE INDEX idx_rides_idempotency ON rides(idempotency_key);

CREATE INDEX idx_trips_ride_id ON trips(ride_id);
CREATE INDEX idx_trips_driver_id ON trips(driver_id);
CREATE INDEX idx_trips_user_id ON trips(user_id);
CREATE INDEX idx_trips_status ON trips(status);

CREATE INDEX idx_payments_trip_id ON payments(trip_id);
CREATE INDEX idx_payments_status ON payments(status);

CREATE INDEX idx_ride_offers_ride_id ON ride_offers(ride_id);
CREATE INDEX idx_ride_offers_driver_id ON ride_offers(driver_id);
CREATE INDEX idx_ride_offers_status ON ride_offers(status);
