using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>The map a project declares: its settings, and the components the game starts with; a project has one class deriving from it.</summary>
    public abstract class Map
    {
        /// <summary>The name of the map; empty keeps the value World Editor set.</summary>
        public string Name { get; protected set; } = "";

        /// <summary>The author of the map; empty keeps the value World Editor set.</summary>
        public string Author { get; protected set; } = "";

        /// <summary>The description of the map; empty keeps the value World Editor set.</summary>
        public string Description { get; protected set; } = "";

        /// <summary>The players the map recommends, as free text such as "Any" or "2-4"; empty keeps the value World Editor set.</summary>
        public string RecommendedPlayers { get; protected set; } = "";

        /// <summary>The components the game starts with, added in list order when the game starts; the constructor lists them.</summary>
        protected List<GameComponent> Components { get; } = new List<GameComponent>();

        /// <summary>The starting components, in the order they were listed.</summary>
        internal List<GameComponent> StartComponents => Components;
    }
}
