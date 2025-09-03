INSERT INTO "products" ("name","code","price","description") VALUES ('Laptop','LAP001',999.99,'High-performance laptop');
SELECT * FROM "products" WHERE (price > 500);
UPDATE "products" SET "price" = 899.99 WHERE "products"."id" = 1;
DELETE FROM "products" WHERE "products"."id" = 1;