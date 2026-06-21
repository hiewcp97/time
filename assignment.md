Take-Home Assignment
Senior Software Engineer — AI & Innovation Team, Time Internet
Context
Time Internet is a broadband provider. Every month a batch of customers’ contracts approach expiry, and
retention agents need to act on them quickly. We want an internal tool that helps agents work this list
efficiently and generate personalised recontract pitches. This is a senior assignment: we care most about
how you design the data layer and backend to stay fast and reliable as the dataset and traffic grow.

The Task
Build a service where a retention agent can:
• Browse, search, filter, and sort customers whose contracts are expiring, with pagination
• View a customer’s details (plan, tenure, usage history)
• Generate a personalised recontract pitch using an LLM that references the customer’s actual data
• Generate pitches in bulk for a filtered segment of customers
A minimal UI is fine — a simple web page or even a well-documented API with a thin frontend. We are not
assessing visual design. The LLM call can be simple. The focus is the backend and data layer.
Data & Scale Requirements
This is the core of the assignment. Build for scale, not just correctness:
• Seed a realistic dataset of at least 500,000 customers with usage history. Provide the seeding script.
• Search, filter, sort, and pagination must stay performant at this volume. Show the indexes you added and
explain why each one exists.
• Include EXPLAIN/ANALYZE output (or equivalent) for your main list query, before and after your
optimisations, in the README.
• Handle the bulk pitch generation so it does not overwhelm the database or the LLM API — show your
approach to batching, concurrency, and backpressure.
• Avoid regenerating a pitch when nothing relevant has changed for a customer. Explain your cache or
idempotency design.
Production Considerations
Address as many as you can in code or in the README. We care about your reasoning and tradeoffs:
• Pagination strategy at scale — offset vs keyset/cursor, and why you chose yours
• Concurrency — what happens when two agents act on the same customer at once
• Failure handling — what happens when a pitch fails mid-batch; the agent should know which succeeded
and which didn’t
• Observability — how you would monitor this in production (logs, metrics, what you’d alert on)
• How the design would change as the dataset grows toward tens of millions of rows (partitioning, sharding,
read replicas — your thinking, not necessarily implemented)
Tech Constraints
• Go or Python for the backend (Go preferred, but use what lets you do your best work)
• PostgreSQL for the data layer
• Any LLM API (OpenAI, Gemini, Claude, open-source)
• Containerize with Docker (docker-compose for local dev)
• Deploy somewhere other than Vercel or Netlify, or document your deployment approach in the README
What We’re Evaluating
• Data-layer engineering — schema, indexing, query design, and performance at scale
• Production thinking — concurrency, failure modes, observability, scaling path
• Architecture decisions and tradeoffs — your README matters as much as the code
• Clean, readable, well-tested code
• Git history — commit frequently so we can see how you work