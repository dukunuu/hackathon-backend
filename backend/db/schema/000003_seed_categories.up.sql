-- ./db/schema/003_seed_initial_categories.up.sql
-- Or if you have a separate seeds directory: ./seeds/001_initial_categories.sql

-- Ensure uuid-ossp extension is enabled if you use uuid_generate_v4() directly in INSERTs
-- and it's not handled by a DEFAULT value in the table definition.
-- If your table has `DEFAULT uuid_generate_v4()` for the id, you don't need to specify `id` here.
-- ./db/schema/003_seed_initial_categories.up.sql
-- Or if you have a separate seeds directory: ./seeds/001_initial_categories.sql

INSERT INTO categories (name, description, endpoint, can_volunteer) VALUES
('Гэрэлтүүлэг', 'Гудамж талбайн гэрэлтүүлэг, шөнийн гэрэлтүүлгийн асуудал', 'lighting', TRUE)
ON CONFLICT (name) DO NOTHING;

INSERT INTO categories (name, description, endpoint, can_volunteer) VALUES
('Явган хүний зам', 'Явган хүний зам, түүний засвар үйлчилгээ, хүртээмж', 'sidewalks', TRUE)
ON CONFLICT (name) DO NOTHING;

INSERT INTO categories (name, description, endpoint, can_volunteer) VALUES
('Машин зам', 'Авто зам, замын нөхцөл байдал, засвар, тэмдэг тэмдэглэгээ', 'roads', FALSE)
ON CONFLICT (name) DO NOTHING;

INSERT INTO categories (name, description, endpoint, can_volunteer) VALUES
('Зогсоол', 'Автомашины зогсоол, түүний хүрэлцээ, зохион байгуулалт', 'parking', FALSE)
ON CONFLICT (name) DO NOTHING;

INSERT INTO categories (name, description, endpoint, can_volunteer) VALUES
('Гэрлэн дохио', 'Замын хөдөлгөөний гэрлэн дохио, түүний ажиллагаа, байршил', 'traffic-lights', FALSE)
ON CONFLICT (name) DO NOTHING;

INSERT INTO categories (name, description, endpoint, can_volunteer) VALUES
('Ногоон байгууламж', 'Цэцэрлэгт хүрээлэн, зүлэгжүүлэлт, мод тарих, арчлах', 'greenery', TRUE)
ON CONFLICT (name) DO NOTHING;

INSERT INTO categories (name, description, endpoint, can_volunteer) VALUES
('Хашаа байшин', 'Барилга байгууламжийн хашаа, хайс, засвар, будах ажил', 'fences-buildings', TRUE)
ON CONFLICT (name) DO NOTHING;

INSERT INTO categories (name, description, endpoint, can_volunteer) VALUES
('Орчны үзэмж', 'Нийтийн эзэмшлийн талбайн цэвэр байдал, тохижилт, гоо зүй', 'ambiance', TRUE)
ON CONFLICT (name) DO NOTHING;

