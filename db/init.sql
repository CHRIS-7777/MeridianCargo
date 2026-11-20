SET NAMES utf8mb4;
CREATE DATABASE IF NOT EXISTS meridian CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'mc_app'@'%' IDENTIFIED BY 'Mc4pp_Db_2024x';
GRANT SELECT, INSERT, UPDATE, DELETE ON meridian.* TO 'mc_app'@'%';
FLUSH PRIVILEGES;
USE meridian;

CREATE TABLE companies (
  id INT AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(120) NOT NULL,
  industry VARCHAR(80),
  billing_email VARCHAR(160),
  country VARCHAR(60)
);

CREATE TABLE users (
  id INT AUTO_INCREMENT PRIMARY KEY,
  company_id INT NOT NULL,
  username VARCHAR(60) NOT NULL UNIQUE,
  email VARCHAR(160) NOT NULL,
  full_name VARCHAR(120),
  phone VARCHAR(40),
  password_hash CHAR(40) NOT NULL,
  role VARCHAR(16) NOT NULL DEFAULT 'customer',
  reset_pin VARCHAR(8) NULL,
  reset_expires DATETIME NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE ports (
  id INT AUTO_INCREMENT PRIMARY KEY,
  code VARCHAR(8) NOT NULL,
  name VARCHAR(100) NOT NULL,
  country VARCHAR(60) NOT NULL,
  region VARCHAR(60) NOT NULL,
  notes VARCHAR(255)
);

CREATE TABLE shipments (
  id INT AUTO_INCREMENT PRIMARY KEY,
  ref VARCHAR(20) NOT NULL UNIQUE,
  company_id INT NOT NULL,
  mode VARCHAR(10) NOT NULL,
  origin_code VARCHAR(8) NOT NULL,
  dest_code VARCHAR(8) NOT NULL,
  status VARCHAR(20) NOT NULL,
  container_no VARCHAR(16),
  weight_kg INT NOT NULL DEFAULT 0,
  eta DATE,
  instructions TEXT,
  created_by INT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE shipment_events (
  id INT AUTO_INCREMENT PRIMARY KEY,
  shipment_id INT NOT NULL,
  event_time DATETIME NOT NULL,
  location VARCHAR(100) NOT NULL,
  description VARCHAR(200) NOT NULL
);

CREATE TABLE invoices (
  id INT AUTO_INCREMENT PRIMARY KEY,
  company_id INT NOT NULL,
  shipment_id INT,
  invoice_no VARCHAR(24) NOT NULL,
  amount_cents INT NOT NULL,
  currency VARCHAR(3) NOT NULL DEFAULT 'USD',
  status VARCHAR(12) NOT NULL,
  due_date DATE,
  billed_to VARCHAR(160)
);

CREATE TABLE documents (
  id INT AUTO_INCREMENT PRIMARY KEY,
  company_id INT NOT NULL,
  title VARCHAR(160) NOT NULL,
  filename VARCHAR(200) NOT NULL,
  source VARCHAR(10) NOT NULL DEFAULT 'vault',
  uploaded_at DATETIME
);

CREATE TABLE integration_keys (
  id INT AUTO_INCREMENT PRIMARY KEY,
  service VARCHAR(60) NOT NULL,
  username VARCHAR(80) NOT NULL,
  secret VARCHAR(120) NOT NULL
);

-- ------------------------------------------------------------------ seed
INSERT INTO companies (id, name, industry, billing_email, country) VALUES
 (1,'Meridian Cargo Ltd','Freight forwarding','finance@meridiancargo.example','Singapore'),
 (2,'Acme Textiles','Apparel','accounts@acme-textiles.example','India'),
 (3,'Globex Marine Supply','Marine equipment','ap@globex-marine.example','Netherlands'),
 (4,'Northline Foods','Food & beverage','payables@northline-foods.example','Canada'),
 (5,'Bluewave Electronics','Consumer electronics','billing@bluewave-el.example','South Korea');

INSERT INTO users (id, company_id, username, email, full_name, phone, password_hash, role) VALUES
 (1,1,'admin','admin@meridiancargo.example','Hana Okafor','+65 6100 0001', SHA1('Fr3ightAdm!n#7'),'admin'),
 (2,1,'ops.desk','ops.desk@meridiancargo.example','Marcus Lindqvist','+65 6100 0014', SHA1('qwerty123'),'staff'),
 (3,1,'dispatch.temp','dispatch.temp@meridiancargo.example','Temp Dispatcher','+65 6100 0019', SHA1('Temp#2024'),'staff'),
 (4,2,'alice','alice.rao@acme-textiles.example','Alice Rao','+91 98450 11223', SHA1('sunshine1'),'customer'),
 (5,2,'bob','bob.menon@acme-textiles.example','Bob Menon','+91 98450 33445', SHA1('bobby123'),'customer'),
 (6,3,'carla','carla.vos@globex-marine.example','Carla Vos','+31 20 555 0142', SHA1('Passw0rd!'),'customer'),
 (7,3,'dmitri','dmitri.k@globex-marine.example','Dmitri Kozlov','+31 20 555 0177', SHA1('iloveships'),'customer'),
 (8,4,'erin','erin.walsh@northline-foods.example','Erin Walsh','+1 604 555 0188', SHA1('erin2024'),'customer'),
 (9,5,'farid','farid.h@bluewave-el.example','Farid Haddad','+82 2 555 0109', SHA1('Electr0nics5'),'customer');

INSERT INTO ports (code, name, country, region, notes) VALUES
 ('SGSIN','Port of Singapore','Singapore','Southeast Asia','24/7 operations. Customs window 6h. Transshipment hub.'),
 ('NLRTM','Port of Rotterdam','Netherlands','Europe','Largest EU port. Pre-arrival notice (ENS) required 24h before call.'),
 ('CNSHA','Port of Shanghai','China','East Asia','Deep-water terminal at Yangshan. Export VGM mandatory.'),
 ('USLAX','Port of Los Angeles','United States','North America','ISF 10+2 filing required 24h before loading.'),
 ('USNYC','Port of New York & New Jersey','United States','North America','Chassis shortages possible during peak season.'),
 ('DEHAM','Port of Hamburg','Germany','Europe','Rail connections to central Europe. Free time 4 days.'),
 ('AEJEA','Jebel Ali','United Arab Emirates','Middle East','Free zone consignments need JAFZA permit.'),
 ('INMAA','Chennai Port','India','South Asia','Textile and automotive cargo. Free time 3 days.'),
 ('INNSA','Nhava Sheva (JNPT)','India','South Asia','Largest container port in India. ICEGATE filing required.'),
 ('GBFXT','Port of Felixstowe','United Kingdom','Europe','Customs clearance via CDS. Free time 5 days.'),
 ('JPYOK','Port of Yokohama','Japan','East Asia','Reefer plugs limited, book early.'),
 ('KRPUS','Port of Busan','South Korea','East Asia','Major transshipment hub for North-East Asia.'),
 ('BEANR','Port of Antwerp-Bruges','Belgium','Europe','Chemical and break-bulk specialist terminals.'),
 ('ESVLC','Port of Valencia','Spain','Europe','Mediterranean gateway. Perishables inspection on site.'),
 ('BRSSZ','Port of Santos','Brazil','South America','Congestion common in harvest season.'),
 ('ZADUR','Port of Durban','South Africa','Africa','Hazardous goods handled at Maydon Wharf only.'),
 ('AUSYD','Port Botany (Sydney)','Australia','Oceania','Biosecurity inspection for timber packaging.'),
 ('CAVAN','Port of Vancouver','Canada','North America','Rail-served. Grain and container terminals.'),
 ('MYPKG','Port Klang','Malaysia','Southeast Asia','Westports and Northport terminals.'),
 ('LKCMB','Port of Colombo','Sri Lanka','South Asia','Transshipment for the Indian subcontinent.'),
 ('EGPSD','Port Said','Egypt','Africa','Suez Canal northern terminus. Canal fees apply.'),
 ('SAJED','Jeddah Islamic Port','Saudi Arabia','Middle East','SABER certificate required for most consumer goods.'),
 ('TRMER','Port of Mersin','Turkey','Middle East','Containers and bulk cargo. Free time 5 days.'),
 ('GRPIR','Port of Piraeus','Greece','Europe','Gateway for south-east Europe via rail.'),
 ('ITGOA','Port of Genoa','Italy','Europe','Strong industrial hinterland.');

INSERT INTO shipments (id, ref, company_id, mode, origin_code, dest_code, status, container_no, weight_kg, eta, instructions, created_by) VALUES
 (1,'MC-2024-10001',2,'Sea','INMAA','NLRTM','In Transit','MSKU4839201',18200, DATE_ADD(CURDATE(), INTERVAL 9 DAY),'Deliver to dock 4, call consignee 30 minutes prior.',4),
 (2,'MC-2024-10002',2,'Sea','INNSA','GBFXT','At Port','TGHU7720418',21500, DATE_ADD(CURDATE(), INTERVAL 3 DAY),'Temperature sensitive dyes. Keep out of direct sun.',4),
 (3,'MC-2024-10003',2,'Air','INMAA','DEHAM','Delivered',NULL,640, DATE_SUB(CURDATE(), INTERVAL 6 DAY),'Sample garments - deliver to showroom reception.',5),
 (4,'MC-2024-10004',3,'Sea','NLRTM','SGSIN','In Transit','CAIU2284756',12400, DATE_ADD(CURDATE(), INTERVAL 14 DAY),'Marine winches. Crane lift required at destination.',6),
 (5,'MC-2024-10005',3,'Sea','DEHAM','AEJEA','Customs Hold','MRKU9034417',9800, DATE_ADD(CURDATE(), INTERVAL 11 DAY),'Dual-use components - export licence attached.',6),
 (6,'MC-2024-10006',3,'Road','ESVLC','ITGOA','Delivered',NULL,3200, DATE_SUB(CURDATE(), INTERVAL 12 DAY),'Deliver Monday-Friday 08:00-16:00 only.',7),
 (7,'MC-2024-10007',4,'Sea','CAVAN','JPYOK','Booked','HLXU6611209',15800, DATE_ADD(CURDATE(), INTERVAL 19 DAY),'Reefer set to -18C. Alarm on deviation.',8),
 (8,'MC-2024-10008',4,'Sea','CAVAN','KRPUS','In Transit','OOLU3317780',16400, DATE_ADD(CURDATE(), INTERVAL 7 DAY),'Frozen seafood - phytosanitary certificate in document vault.',8),
 (9,'MC-2024-10009',4,'Air','CAVAN','SGSIN','Delayed',NULL,820, DATE_ADD(CURDATE(), INTERVAL 2 DAY),'Perishable samples, priority handling.',8),
 (10,'MC-2024-10010',5,'Sea','KRPUS','USLAX','In Transit','EGHU1049387',11200, DATE_ADD(CURDATE(), INTERVAL 8 DAY),'Lithium batteries class 9 - declare DG on arrival.',9),
 (11,'MC-2024-10011',5,'Air','KRPUS','USNYC','Delivered',NULL,455, DATE_SUB(CURDATE(), INTERVAL 3 DAY),'High value electronics - signature required.',9),
 (12,'MC-2024-10012',5,'Sea','KRPUS','BEANR','Customs Hold','TCLU5520913',13900, DATE_ADD(CURDATE(), INTERVAL 16 DAY),'CE documentation requested by customs.',9),
 (13,'MC-2024-10013',2,'Sea','INNSA','USNYC','Booked','FCIU8801326',19200, DATE_ADD(CURDATE(), INTERVAL 24 DAY),'Palletised, do not double stack.',4),
 (14,'MC-2024-10014',3,'Air','NLRTM','SAJED','In Transit',NULL,1180, DATE_ADD(CURDATE(), INTERVAL 1 DAY),'SABER certificate emailed to consignee.',7);

INSERT INTO shipment_events (shipment_id, event_time, location, description) VALUES
 (1, DATE_SUB(NOW(), INTERVAL 12 DAY),'Chennai Port','Container gated in'),
 (1, DATE_SUB(NOW(), INTERVAL 10 DAY),'Chennai Port','Loaded on vessel MV Aurora Bay'),
 (1, DATE_SUB(NOW(), INTERVAL 3 DAY),'Colombo','Transshipment completed'),
 (2, DATE_SUB(NOW(), INTERVAL 20 DAY),'Nhava Sheva','Container gated in'),
 (2, DATE_SUB(NOW(), INTERVAL 2 DAY),'Port of Felixstowe','Vessel arrived, discharge pending'),
 (3, DATE_SUB(NOW(), INTERVAL 9 DAY),'Chennai Airport','Cargo accepted'),
 (3, DATE_SUB(NOW(), INTERVAL 6 DAY),'Hamburg Airport','Delivered to consignee'),
 (4, DATE_SUB(NOW(), INTERVAL 15 DAY),'Port of Rotterdam','Loaded on vessel MV Northern Pearl'),
 (5, DATE_SUB(NOW(), INTERVAL 5 DAY),'Jebel Ali','Held by customs - documentation review'),
 (8, DATE_SUB(NOW(), INTERVAL 8 DAY),'Port of Vancouver','Loaded on vessel MV Pacific Dawn'),
 (9, DATE_SUB(NOW(), INTERVAL 1 DAY),'Vancouver Airport','Flight rescheduled, new departure +24h'),
 (10, DATE_SUB(NOW(), INTERVAL 7 DAY),'Port of Busan','Loaded on vessel MV Han River'),
 (12, DATE_SUB(NOW(), INTERVAL 4 DAY),'Antwerp-Bruges','Held by customs - CE documentation');

INSERT INTO invoices (id, company_id, shipment_id, invoice_no, amount_cents, currency, status, due_date, billed_to) VALUES
 (1,2,1,'INV-2024-30001',412500,'USD','Paid', DATE_SUB(CURDATE(), INTERVAL 20 DAY),'accounts@acme-textiles.example'),
 (2,2,2,'INV-2024-30002',538000,'USD','Open', DATE_ADD(CURDATE(), INTERVAL 12 DAY),'accounts@acme-textiles.example'),
 (3,2,3,'INV-2024-30003',87250,'USD','Paid', DATE_SUB(CURDATE(), INTERVAL 5 DAY),'accounts@acme-textiles.example'),
 (4,3,4,'INV-2024-30004',629900,'EUR','Open', DATE_ADD(CURDATE(), INTERVAL 20 DAY),'ap@globex-marine.example'),
 (5,3,5,'INV-2024-30005',301750,'EUR','Open', DATE_ADD(CURDATE(), INTERVAL 8 DAY),'ap@globex-marine.example'),
 (6,3,6,'INV-2024-30006',148000,'EUR','Paid', DATE_SUB(CURDATE(), INTERVAL 14 DAY),'ap@globex-marine.example'),
 (7,4,7,'INV-2024-30007',472300,'CAD','Open', DATE_ADD(CURDATE(), INTERVAL 25 DAY),'payables@northline-foods.example'),
 (8,4,8,'INV-2024-30008',455100,'CAD','Open', DATE_ADD(CURDATE(), INTERVAL 10 DAY),'payables@northline-foods.example'),
 (9,4,9,'INV-2024-30009',219900,'CAD','Open', DATE_ADD(CURDATE(), INTERVAL 5 DAY),'payables@northline-foods.example'),
 (10,5,10,'INV-2024-30010',388800,'USD','Open', DATE_ADD(CURDATE(), INTERVAL 16 DAY),'billing@bluewave-el.example'),
 (11,5,11,'INV-2024-30011',132400,'USD','Paid', DATE_SUB(CURDATE(), INTERVAL 2 DAY),'billing@bluewave-el.example'),
 (12,5,12,'INV-2024-30012',404000,'USD','Open', DATE_ADD(CURDATE(), INTERVAL 30 DAY),'billing@bluewave-el.example'),
 (13,2,13,'INV-2024-30013',601000,'USD','Open', DATE_ADD(CURDATE(), INTERVAL 40 DAY),'accounts@acme-textiles.example'),
 (14,3,14,'INV-2024-30014',92300,'EUR','Open', DATE_ADD(CURDATE(), INTERVAL 6 DAY),'ap@globex-marine.example');

INSERT INTO documents (company_id, title, filename, source, uploaded_at) VALUES
 (2,'Bill of lading MC-2024-10001','bol_MC-2024-10001.txt','vault', DATE_SUB(NOW(), INTERVAL 12 DAY)),
 (2,'Packing list - dyes batch 7','packing_list_acme_0007.txt','vault', DATE_SUB(NOW(), INTERVAL 21 DAY)),
 (3,'Export licence - dual use','export_licence_globex_5521.txt','vault', DATE_SUB(NOW(), INTERVAL 6 DAY)),
 (3,'Customs declaration 2231','customs_declaration_2231.txt','vault', DATE_SUB(NOW(), INTERVAL 6 DAY)),
 (4,'Phytosanitary certificate','phyto_cert_northline_88.txt','vault', DATE_SUB(NOW(), INTERVAL 9 DAY)),
 (5,'Commercial invoice KR-7781','commercial_invoice_KR-7781.txt','vault', DATE_SUB(NOW(), INTERVAL 8 DAY)),
 (5,'Battery test summary UN38.3','battery_test_UN383.txt','vault', DATE_SUB(NOW(), INTERVAL 8 DAY));

INSERT INTO integration_keys (service, username, secret) VALUES
 ('carrier-edi','edi_sync','Edi$ync-7731-Rq'),
 ('customs-gateway','mc_filer','Cstm#Filer-2291'),
 ('payments-sandbox','pk_test_meridian','sk_test_51Mq9ZcKkT7wLx2hD0aVnE4yU'),
 ('warehouse-wms','wms_bridge','Wms!Bridge-4408-zA');
