namespace Moonwell.Testing
{
    /// <summary>A unit that is both a damage source, writing IDamageSource.OnLanded(Hit), and a push target, writing IPushTarget.OnGrounded(float): it writes "damage landed" and "grounded" to the test log.</summary>
    public class GroundedSource : Unit, IDamageSource, IPushTarget
    {
        /// <summary>The speed of each coming down.</summary>
        public System.Collections.Generic.List<float> Groundings = new System.Collections.Generic.List<float>();

        /// <summary>Logs the damage reaction.</summary>
        public void OnLanded(Hit hit) => TestLog.Lines.Add("damage landed");

        /// <summary>Logs the push reaction and its speed.</summary>
        public void OnGrounded(float speed)
        {
            TestLog.Lines.Add("grounded");
            Groundings.Add(speed);
        }
    }
}
