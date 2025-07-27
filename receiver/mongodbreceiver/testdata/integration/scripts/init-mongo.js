// Create database and collections
db = db.getSiblingDB('sample_db');

// Create a collection
db.createCollection('users');

// Create a user with read/write access to the database
db.createUser({
  user: 'app_user',
  pwd: 'app_password',
  roles: [{
    role: 'readWrite',
    db: 'sample_db'
  }]
});

// TODO: CREATE A OTEL USER