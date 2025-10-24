-- Add missing user_product_subscriptions join table
-- This is a many-to-many relationship between users and products
CREATE TABLE IF NOT EXISTS user_product_subscriptions (
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    product_id INTEGER REFERENCES products(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, product_id)
);
