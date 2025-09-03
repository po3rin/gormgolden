INSERT INTO `users` (`name`,`email`,`age`) VALUES ("John Doe","john@example.com",30) RETURNING `id`;
SELECT * FROM `users` WHERE age > 25;
UPDATE `users` SET `age`=31 WHERE `id` = 1;
DELETE FROM `users` WHERE `users`.`id` = 1;