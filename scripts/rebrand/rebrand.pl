#!/usr/bin/env perl
# rebrand.pl - apply map.tsv to files in one pass.
#   rebrand.pl --apply <file>...   rewrite files whose content changes (mtime untouched otherwise)
#   rebrand.pl --check <file>...   print each changed line as "file:line: old" / "+ new"; exit 1 if any
# Replacement column: literal text; $1..$9 stand for capture groups. Binary files (NUL byte) and
# files that are not valid UTF-8 are skipped with a warning. Exit 2 on usage or a missing file.
use strict; use warnings; use utf8;
use Encode qw(decode encode FB_CROAK);
use File::Basename qw(dirname);
use Cwd ();
binmode STDOUT, ':encoding(UTF-8)'; binmode STDERR, ':encoding(UTF-8)';

my $mode = shift // '';
die "usage: $0 --apply|--check <file>...\n" unless $mode =~ /^--(apply|check)$/ && @ARGV;
my $map = dirname(__FILE__) . '/map.tsv';
open my $m, '<:encoding(UTF-8)', $map or die "$map: $!\n";
my @rules;
while (<$m>) { chomp; next if /^#/ || /^\s*$/; my ($re, $to) = split /\t/, $_, 2; push @rules, [qr/$re/, $to]; }
close $m;

sub rewrite {
  my ($s) = @_;
  for my $r (@rules) {
    my ($re, $to) = @$r;
    $s =~ s/$re/my @g = ($1, $2, $3, $4, $5, $6, $7, $8, $9); (my $t = $to) =~ s{\$([1-9])(?!\d)}{$g[$1 - 1] \/\/ ""}ge; $t/ge;
  }
  return $s;
}

# Docs-lead phase: --apply may only touch website-docs/ inside this repo (files outside the repo, such as
# scratch copies, are fine). Set REBRAND_ALLOW_CODE=1 when the code rebrand starts, then delete this guard.
my $root = `git rev-parse --show-toplevel 2>/dev/null`; chomp $root;
sub guarded {
  my ($f) = @_;
  return 0 if $mode ne '--apply' || $ENV{REBRAND_ALLOW_CODE} || $root eq '';
  my $abs = Cwd::abs_path($f) // return 0;
  return index($abs, "$root/") == 0 && index($abs, "$root/website-docs/") != 0;
}

my $changed = 0;
for my $f (@ARGV) {
  -f $f or do { print STDERR "$f: not a file\n"; exit 2 };
  guarded($f) and do { print STDERR "$f: --apply is limited to website-docs/ during the docs-lead phase (set REBRAND_ALLOW_CODE=1 for the code rebrand)\n"; exit 2 };
  open my $in, '<:raw', $f or do { print STDERR "$f: $!\n"; exit 2 };
  local $/; my $bytes = <$in>; close $in;
  if (index($bytes, "\0") >= 0) { print STDERR "$f: binary, skipped\n"; next }
  my $text = eval { decode('UTF-8', $bytes, FB_CROAK) };
  if (!defined $text) { print STDERR "$f: not valid UTF-8, skipped\n"; next }
  my $out = rewrite($text);
  next if $out eq $text;
  $changed++;
  if ($mode eq '--apply') {
    open my $o, '>:raw', $f or die "$f: $!\n"; print {$o} encode('UTF-8', $out); close $o;
  } else {
    my @a = split /\n/, $text, -1; my @b = split /\n/, $out, -1;
    for my $i (0 .. $#a) {
      next if defined $b[$i] && $a[$i] eq $b[$i];
      printf "%s:%d: %s\n+ %s\n", $f, $i + 1, $a[$i], $b[$i] // '';
    }
  }
}
exit($mode eq '--check' && $changed ? 1 : 0);
