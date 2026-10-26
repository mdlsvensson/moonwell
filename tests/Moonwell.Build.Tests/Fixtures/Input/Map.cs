using Moonwell;

namespace InputFixture
{
    /// <summary>A map that uses the input package: a camera tour at the start, and a hero that turns to the cursor.</summary>
    public sealed class InputMap : Map
    {
        public InputMap()
        {
            Name = "Input";
            Components.Add(new Tour());
        }
    }
}
