namespace Moonwell.Testing
{
    /// <summary>A Dummy that adds two components in one call after the DummyComp its parent listed.</summary>
    public sealed class DummyAddMany : Dummy
    {
        /// <summary>Adds a Tally and a StrikerComp.</summary>
        public DummyAddMany()
        {
            Components.Add(new Tally(), new StrikerComp());
        }
    }
}
