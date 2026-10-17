namespace Moonwell.Testing
{
    /// <summary>A destructable that starts with a GatePart, and logs when it dies.</summary>
    public class TestGate : Destructable
    {
        /// <summary>Makes a gate that starts with a GatePart.</summary>
        public TestGate()
        {
            Components.Add(new GatePart());
        }

        /// <summary>Logs that the gate died.</summary>
        public override void OnDeath()
        {
            TestLog.Lines.Add("gate died");
        }
    }
}
