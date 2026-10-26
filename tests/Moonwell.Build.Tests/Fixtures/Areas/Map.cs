using Moonwell;

namespace AreasFixture
{
    /// <summary>A map that uses the areas package: a town that adds a shop floor on one of the map's two regions.</summary>
    public sealed class AreasMap : Map
    {
        public AreasMap()
        {
            Name = "Areas";
            Components.Add(new Town());
        }
    }
}
