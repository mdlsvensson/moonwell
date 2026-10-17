namespace Moonwell.Testing
{
    /// <summary>An item that starts with an OrbGlow, and logs when it is picked up.</summary>
    public class TestOrb : Item
    {
        /// <summary>Makes an orb that starts with an OrbGlow.</summary>
        public TestOrb()
        {
            Components.Add(new OrbGlow());
        }

        /// <summary>Logs that the orb was picked up.</summary>
        public override void OnPickedUp(Unit by)
        {
            TestLog.Lines.Add("orb picked");
        }
    }
}
