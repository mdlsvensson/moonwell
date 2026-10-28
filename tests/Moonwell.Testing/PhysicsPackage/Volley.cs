namespace Moonwell.Testing
{
    /// <summary>A buff that fires a dart from its unit toward (1000, 0) when it is applied, with the owner a case gives it, or none.</summary>
    public sealed class Volley : Buff
    {
        /// <summary>The owner the dart is fired with; null for none.</summary>
        public Owner? Given;

        /// <summary>The dart fired; null before OnApply.</summary>
        public Dart? Fired;

        /// <summary>The owner that was current inside OnApply.</summary>
        public Owner? Seen;

        /// <summary>Fires the dart.</summary>
        public override void OnApply()
        {
            Seen = Owner.Current;
            var dart = new Dart();
            dart.Speed = 100.0f;
            Fired = Unit.Fire(dart, 1000.0f, 0.0f, Given);
        }
    }
}
