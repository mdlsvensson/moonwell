namespace Moonwell.Testing
{
    /// <summary>A map that assigns its starting components as one list: TestGameB, then TestGameA.</summary>
    public sealed class TestMapAssigned : Map
    {
        /// <summary>Assigns the list.</summary>
        public TestMapAssigned()
        {
            Components.Add(new TestGameA());
            Components = new() { new TestGameB(), new TestGameA() };
        }
    }
}
