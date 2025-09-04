INSERT INTO `users` (`name`,`email`,`age`) VALUES ("Alice","alice@example.com",28) RETURNING `id`;
SELECT * FROM `users` WHERE `age`>25;
DELETE FROM `users` WHERE `users`.`id`=1;