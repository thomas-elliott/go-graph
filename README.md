# go-graph

Example / testing project for a go-backed knowledge graph.

Eventual goals for this are to be able to show how a GraphRAG system works
and to optimise one for some current use cases.

Starting with an in memory node then building out to using a pgvector/neo4j database

# Sample Data

Sample data in ./data is from https://dumps.wikimedia.org/enwikivoyage

Loaded and transformed into nodes:

```json
{"id":"Bruges","labels":["destination","city"],
 "properties":{"title":"Bruges","status":"guide","lat":51.2094,"lon":3.2249,"summary":"..."}}
{"id":"poi:Bruges#0","labels":["poi","see"],
 "properties":{"name":"Grote Markt","dest":"Bruges","content":"...","lat":51.2087,"lon":3.2244}}
```

and edges:

```json
{"from":"Bruges","to":"West Flanders","type":"part_of"}
{"from":"Bruges","to":"Ghent","type":"link","properties":{"count":3,"sections":["Go next","Get in"]}}
{"from":"Bruges","to":"poi:Bruges#0","type":"has_poi"}
```
