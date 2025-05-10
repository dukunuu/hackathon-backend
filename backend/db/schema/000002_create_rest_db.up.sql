CREATE TYPE post_status AS ENUM (
    'Шийдвэрлэгдсэн',
    'Түр завсарласан',
    'Цуцлагдсан',
    'Шийдвэрлэгдэж байгаа',
    'Хүлээгдэж байгаа'
);

CREATE TYPE post_priority AS ENUM (
    'өндөр',
    'дунд',
    'бага'
);

CREATE TYPE post_type AS ENUM (
    'хандив',
    'гомдол'
);

CREATE TYPE volunteer_status AS ENUM('pending', 'approved', 'completed', 'rejected');

CREATE TABLE IF NOT EXISTS categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    endpoint TEXT NOT NULL UNIQUE,
    can_volunteer BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS posts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title VARCHAR(150) NOT NULL,
    description TEXT NOT NULL,
    status post_status NOT NULL DEFAULT 'Хүлээгдэж байгаа',
    priority post_priority NOT NULL DEFAULT 'бага',
    preview_url TEXT,
    post_type post_type NOT NULL DEFAULT 'гомдол',
    user_id UUID NOT NULL,
    max_volunteers INT NOT NULL DEFAULT 0,
    current_volunteers INT NOT NULL DEFAULT 0,
    category_id UUID,
    location_lat DOUBLE PRECISION,
    location_lng DOUBLE PRECISION,
    address_text TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_category FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS post_images (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    post_id UUID NOT NULL,
    image_url TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_post FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS post_volunteers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL,
    post_id UUID NOT NULL,
    status VARCHAR(50) DEFAULT 'pending', -- e.g., 'pending', 'approved', 'rejected', 'completed'
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_post FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    UNIQUE (user_id, post_id)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_posts_user_id ON posts(user_id);
CREATE INDEX IF NOT EXISTS idx_posts_category_id ON posts(category_id);
CREATE INDEX IF NOT EXISTS idx_posts_status ON posts(status);
CREATE INDEX IF NOT EXISTS idx_posts_post_type ON posts(post_type);
CREATE INDEX IF NOT EXISTS idx_post_images_post_id ON post_images(post_id);
CREATE INDEX IF NOT EXISTS idx_post_volunteers_post_id ON post_volunteers(post_id);
CREATE INDEX IF NOT EXISTS idx_post_volunteers_user_id ON post_volunteers(user_id);
