-- =============================================================================
-- HW8 STEP I: Shopping Cart MySQL Schema
-- =============================================================================
--
-- DESIGN DECISIONS (for your report):
--
-- 1. TABLE STRUCTURE: Two tables (carts + cart_items)
--    - Normalized design: cart metadata separate from items
--    - Follows 3NF: no redundant data
--    - Trade-off: requires JOIN for full cart retrieval, but indexed JOINs
--      are fast and this preserves data integrity
--
-- 2. KEY STRATEGY:
--    - cart_id: CHAR(36) UUID — globally unique across distributed ECS tasks
--    - cart_items: auto-increment PK + (cart_id, product_id) unique constraint
--    - Foreign key with CASCADE DELETE prevents orphaned items
--
-- 3. INDEX STRATEGY:
--    - idx_carts_customer: customer history queries
--    - uk_cart_product: prevents duplicate items + speeds up item lookups
--    - Foreign key index on cart_id: fast JOINs for GET /shopping-carts/{id}
--
-- 4. TRANSACTION DESIGN:
--    - Add-item: INSERT ... ON DUPLICATE KEY UPDATE (atomic upsert)
--    - InnoDB row-level locking for concurrent cart modifications
--
-- =============================================================================

CREATE DATABASE IF NOT EXISTS shopping_cart_db;
USE shopping_cart_db;

CREATE TABLE IF NOT EXISTS carts (
    cart_id       CHAR(36)     NOT NULL,
    customer_id   VARCHAR(100) NOT NULL,
    customer_name VARCHAR(255) DEFAULT NULL,
    created_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (cart_id),
    INDEX idx_carts_customer (customer_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS cart_items (
    item_id       BIGINT       NOT NULL AUTO_INCREMENT,
    cart_id       CHAR(36)     NOT NULL,
    product_id    VARCHAR(100) NOT NULL,
    product_name  VARCHAR(255) DEFAULT NULL,
    quantity      INT          NOT NULL DEFAULT 1,
    unit_price    DECIMAL(10,2) NOT NULL DEFAULT 0.00,
    added_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (item_id),
    UNIQUE KEY  uk_cart_product (cart_id, product_id),
    INDEX       idx_items_cart_id (cart_id),

    CONSTRAINT fk_items_cart
        FOREIGN KEY (cart_id) REFERENCES carts(cart_id)
        ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
