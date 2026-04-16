ALTER TABLE webhook_delivery
    DROP CONSTRAINT webhook_delivery_subscription_id_fkey,
    ADD CONSTRAINT webhook_delivery_subscription_id_fkey
        FOREIGN KEY (subscription_id)
        REFERENCES webhook_subscription (id)
        ON DELETE CASCADE;
