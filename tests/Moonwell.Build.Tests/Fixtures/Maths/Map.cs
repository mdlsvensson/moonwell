using Moonwell;

namespace MathsFixture
{
    /// <summary>A map that uses the maths package: a footman pushed along a line at the start, unless a wall is in the way.</summary>
    public sealed class MathsMap : Map
    {
        public MathsMap()
        {
            Name = "Maths";
            Components.Add(new Push());
        }
    }
}
