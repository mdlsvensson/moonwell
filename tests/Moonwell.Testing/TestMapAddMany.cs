namespace Moonwell.Testing
{
    /// <summary>A map that adds three starting components in one call: TestGameA, TestGameB, TestGameA.</summary>
    public sealed class TestMapAddMany : Map
    {
        /// <summary>Adds the three.</summary>
        public TestMapAddMany()
        {
            Components.Add(new TestGameA(), new TestGameB(), new TestGameA());
        }
    }
}
