CREATE TABLE "peeple_users" (
    id SERIAL PRIMARY KEY,
    phone_number VARCHAR(10) NOT NULL UNIQUE
);
