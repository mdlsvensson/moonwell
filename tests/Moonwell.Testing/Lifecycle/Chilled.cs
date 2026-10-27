namespace Moonwell.Testing
{
    /// <summary>A buff whose duration and strength its constructor takes, and which counts its applies.</summary>
    public sealed class Chilled : Buff
    {
        /// <summary>How strong the chill is; a value of the buff's own, which a refresh keeps.</summary>
        public float Strength;

        /// <summary>How many times OnApply has run on this buff.</summary>
        public int Applied;

        /// <summary>Makes a chill for the seconds (0 lasts until it is removed) at a strength.</summary>
        public Chilled(float seconds, float strength)
        {
            Duration = seconds;
            Strength = strength;
        }

        /// <summary>Counts the apply.</summary>
        public override void OnApply()
        {
            Applied++;
        }
    }
}
