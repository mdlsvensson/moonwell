namespace Moonwell.Testing
{
    /// <summary>A buff that lasts 3 seconds and applies itself again the first time it expires.</summary>
    public class ReapplyBurn : Buff
    {
        /// <summary>Whether the buff has applied itself again already.</summary>
        private bool reapplied;

        /// <summary>Makes a buff that lasts 3 seconds.</summary>
        public ReapplyBurn()
        {
            Duration = 3f;
        }

        /// <summary>Applies the buff again the first time it expires.</summary>
        public override void OnExpire()
        {
            if (!reapplied)
            {
                reapplied = true;
                Unit.Apply(this, Source);
            }
        }
    }
}
