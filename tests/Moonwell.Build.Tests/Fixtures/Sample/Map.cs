using Moonwell;

namespace Sample
{
    /// <summary>The sample map's declaration.</summary>
    public sealed class SampleMap : Map
    {
        public SampleMap()
        {
            Name = "Sample map";
            Author = "Moonwell";
            Description = "Built by the tests.";
            RecommendedPlayers = "Any";
            Components.Add(new Hello());
        }
    }
}
