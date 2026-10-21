# go-graph

A go-backed knowledge graph using WikiVoyage as the sample data, and several different search
implementations with an eval harness to compare the implementations on accuracy and speed.

This project shows the retrieval layer of a GraphRAG system, to demonstrate the different 
techniques and for optimising/testing the different strategies and queries.

Pre-requisites:
- just
- docker
- go 1.26
- postgres 18 with pgvector 0.8.0 (created by `just db-up`)

# Quick Start

Install go and just (via mise), create the local db (needs docker), load the sample data, run the example query:

```
mise install
just db-up
just load
just dev
```

# Example output

This is run with the full embedded dataset.

```
go run ./cmd/example
memory graph has 393 nodes
text search: bruges trains
  0.762  chunk:Bruges#2.0
  0.630  chunk:Belgian coast by tram#3.0
  0.352  chunk:Bruges#11
  0.340  chunk:Knokke-Heist#2
  0.282  chunk:Roeselare#1
vector search: bruges trains
  0.890  chunk:Baiersbronn#2
  0.886  chunk:Bergen#3.2
  0.885  chunk:Barcelona#4.2
  0.881  chunk:Belgrade#2.1
  0.880  chunk:Bergen#2.0
text search: how do I get to Bruges by rail
  0.833  chunk:Bruges#2.0
  0.183  chunk:Ghent#10
  0.146  chunk:Belgian coast by tram#3.0
  0.009  chunk:Ghent#3
  0.002  chunk:Eupen#2
vector search: how do I get to Bruges by rail
  0.920  chunk:Belgium#6.1
  0.914  chunk:Belgian coast by tram#3.0
  0.911  chunk:Belgian coast by tram#3.1
  0.909  chunk:Belgium#7.0
  0.906  chunk:Belgian coast by tram#10
memory: part of Belgium: [Brussels Flanders]
memory: around chunk:Bruges#2: [{chunk:Bruges#2 0}]
postgres: part of Belgium: [Brussels Flanders Wallonia]
postgres: around chunk:Bruges#2: [{chunk:Bruges#2 0}]
```

# Architecture

`graph.Graph` is the interface for the GraphRAG system with different backing implementations.
Currently implemented with in-memory and postgres.

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

# Embeddings

Uses any server with OpenAI style embeddings 

Vector search needs an embedding server with an OpenAI-style `/v1/embeddings`
endpoint; none is bundled. Copy `.env.example` to `.env` and set the URL, model,
dimensions and API key, then run `just embed`.

Every vector fetched is also kept in `embedding_cache`, keyed by model, size and
a hash of the embedded text.

# Next

Support for Neo4J to test it against the Postgres implementation.