namespace Moonwell.Testing
{
    /// <summary>A map that sets none of its settings and lists two starting components, TestGameA then TestGameB.</summary>
    public sealed class TestMap : Map
    {
        /// <summary>Lists the two components in the order the game starts them.</summary>
        public TestMap()
        {
            Components.Add(new TestGameA());
            Components.Add(new TestGameB());
        }
    }
}
