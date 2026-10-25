using Moonwell;

namespace TerrainFixture
{
    /// <summary>A map that uses the terrain package: a footman made at the start and a read of the ground under it a moment later.</summary>
    public sealed class TerrainMap : Map
    {
        public TerrainMap()
        {
            Name = "Terrain";
            Components.Add(new Probe());
        }
    }
}
