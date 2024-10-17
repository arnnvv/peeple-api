CREATE TABLE peeple_api_users (
    id VARCHAR PRIMARY KEY,
    name VARCHAR,
    email VARCHAR NOT NULL UNIQUE,
    location VARCHAR(255),
    gender VARCHAR,
    relationshiptype VARCHAR,
    height INTEGER,
    religion VARCHAR,
    occupation_field VARCHAR(255),
    occupation_area VARCHAR,
    drink VARCHAR,
    smoke VARCHAR,
    bio TEXT,
    date INTEGER,
    month INTEGER,
    year INTEGER,
    subscription VARCHAR DEFAULT 'free',
    instaid VARCHAR,
    phone VARCHAR
);

CREATE TABLE peeple_api_pictures (
    id SERIAL PRIMARY KEY,
    email VARCHAR NOT NULL,
    url VARCHAR NOT NULL,
    FOREIGN KEY (email) REFERENCES peeple_api_users(email)
);

CREATE TABLE peeple_api_likes (
    id SERIAL PRIMARY KEY,
    likerEmail VARCHAR NOT NULL,
    likedEmail VARCHAR NOT NULL,
    FOREIGN KEY (likerEmail) REFERENCES peeple_api_users(email),
    FOREIGN KEY (likedEmail) REFERENCES peeple_api_users(email)
);

CREATE TABLE peeple_api_matches (
    id SERIAL PRIMARY KEY,
    user1id VARCHAR NOT NULL,
    user2id VARCHAR NOT NULL,
    matchedat TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user1id) REFERENCES peeple_api_users(id),
    FOREIGN KEY (user2id) REFERENCES peeple_api_users(id)
);

CREATE TABLE peeple_api_messages (
    id SERIAL PRIMARY KEY,
    senderEmail VARCHAR NOT NULL,
    receiverEmail VARCHAR NOT NULL,
    content TEXT NOT NULL,
    sentat TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    isread BOOLEAN DEFAULT FALSE,
    FOREIGN KEY (senderEmail) REFERENCES peeple_api_users(email),
    FOREIGN KEY (receiverEmail) REFERENCES peeple_api_users(email)
);

CREATE TABLE peeple_api_userpreferences (
    id SERIAL PRIMARY KEY,
    userid VARCHAR NOT NULL,
    agerange JSONB,
    genderpreference JSONB,
    relationshiptypepreference JSONB,
    maxdistance INTEGER,
    FOREIGN KEY (userid) REFERENCES peeple_api_users(id)
);

CREATE TABLE peeple_api_profileImages (
    id SERIAL PRIMARY KEY,
    email VARCHAR NOT NULL,
    url VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    "imageNo" INTEGER NOT NULL,
    FOREIGN KEY (email) REFERENCES peeple_api_users(email)
);
