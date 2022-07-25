#!/bin/bash
set -e

add_collections() {
    mongo <<EOF
    use testdb
    db.createCollection("orders")
    db.createCollection("products")
    db.products.insert( { item: "envelopes", qty : 100, type: "Clasp" }, { writeConcern: { w: 1, wtimeout: 5000 } } )
    db.orders.createIndex( { item: 1, quantity: 1 } )
    db.orders.createIndex( { type: 1, item: 1 } )
    { "_id" : 1, "item" : "abc", "price" : 12, "quantity" : 2, "type": "apparel" }
    { "_id" : 2, "item" : "jkl", "price" : 20, "quantity" : 1, "type": "electronics" }
    { "_id" : 3, "item" : "abc", "price" : 10, "quantity" : 5, "type": "apparel" }
    db.orders.find( { type: "apparel"} )
    db.orders.find( { item: "abc" } ).sort( { quantity: 1 } )
EOF
}

echo "Adding collections. . ."
end=$((SECONDS+20))
while [ $SECONDS -lt $end ]; do
    if add_collections; then
        echo "collections added!"
        exit 0
    fi
    echo "Trying again in 5 seconds. . ."
    sleep 5
done

echo "Failed to add collections"
