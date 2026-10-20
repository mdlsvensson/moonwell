namespace Moonwell.Testing
{
    /// <summary>A Dummy that assigns its components as a new list, which drops the DummyComp its parent listed.</summary>
    public sealed class DummyAssigned : Dummy
    {
        /// <summary>Assigns a Tally as the only component.</summary>
        public DummyAssigned()
        {
            Components = new() { new Tally() };
        }
    }
}
