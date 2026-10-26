using Moonwell;

namespace InteractionsFixture
{
    /// <summary>A map that uses the interactions package: a well that burns the units near it, and a search a second after the start.</summary>
    public sealed class InteractionsMap : Map
    {
        public InteractionsMap()
        {
            Name = "Interactions";
            Components.Add(new Wells());
        }
    }
}
