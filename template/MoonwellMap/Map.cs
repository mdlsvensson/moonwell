using Moonwell;

namespace MoonwellMap
{
    /// <summary>The map's settings, and the components the game starts with.</summary>
    public sealed class Settings : Map
    {
        public Settings()
        {
            Name = "MoonwellMap";
            Author = "";
            Description = "A map made with Moonwell.";
            RecommendedPlayers = "1";
            Components.Add(new Start());
        }
    }
}
