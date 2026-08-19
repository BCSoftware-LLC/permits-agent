"""Ad-hoc MCP client smoke test for the abc-agent server (stdio transport)."""
import asyncio
import json

from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client


def show(result):
    for c in result.content:
        if c.type == "text":
            print(c.text[:900])
        else:
            print(c)


async def main():
    params = StdioServerParameters(command="abc-agent", args=["serve"])
    async with stdio_client(params) as (read, write):
        async with ClientSession(read, write) as session:
            await session.initialize()
            tools = await session.list_tools()
            names = [t.name for t in tools.tools]
            print(f"TOOLS ({len(names)}): {names}\n")

            print("=== call license_stats ===")
            show(await session.call_tool("license_stats", {}))

            print("\n=== call search_licenses('stater bros', limit=2) ===")
            show(await session.call_tool("search_licenses", {"query": "stater bros", "limit": 2}))

            print("\n=== call pending_applications(city='HOLLYWOOD', limit=2) ===")
            show(await session.call_tool("pending_applications", {"city": "HOLLYWOOD", "limit": 2}))

            print("\n=== call expiring_licenses(days=45, county='LOS ANGELES', limit=2) ===")
            show(await session.call_tool("expiring_licenses", {"days": 45, "county": "LOS ANGELES", "limit": 2}))

            print("\n=== call license_type_description('47') ===")
            show(await session.call_tool("license_type_description", {"code": "47"}))

            print("\n=== call search_forms('transfer') ===")
            show(await session.call_tool("search_forms", {"query": "transfer"}))

            print("\n=== call latest_news(feed='advisories', limit=2) ===")
            show(await session.call_tool("latest_news", {"feed": "advisories", "limit": 2}))

            print("\n=== call fee_surcharges ===")
            show(await session.call_tool("fee_surcharges", {}))

            print("\n=== resources ===")
            resources = await session.list_resources()
            print([r.uri for r in resources.resources])


asyncio.run(main())
