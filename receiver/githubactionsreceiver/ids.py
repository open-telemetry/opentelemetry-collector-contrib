import hashlib
import binascii

def generate_trace_id(run_id, run_attempt):
    input_str = f"{run_id}{run_attempt}t"
    hashed = hashlib.sha256(input_str.encode()).digest()
    return binascii.hexlify(hashed).decode()[:32]

def generate_parent_span_id(job_id, run_attempt):
    input_str = f"{job_id}{run_attempt}s"
    hashed = hashlib.sha256(input_str.encode()).digest()
    return binascii.hexlify(hashed).decode()[:16]

def generate_span_id(job_id, run_attempt, step_name, step_number=None):
    step_number_str = str(step_number) if step_number is not None else ""
    input_str = f"{job_id}{run_attempt}{step_name}{step_number_str}"
    hashed = hashlib.sha256(input_str.encode()).digest()
    return binascii.hexlify(hashed).decode()[:16]

# Example usage
trace_id = generate_trace_id(12345, 1)
parent_span_id = generate_parent_span_id(54321, 1)
span_id = generate_span_id(54321, 1, "step-name", 1)

print("Trace ID:", trace_id)
print("Parent Span ID:", parent_span_id)
print("Span ID:", span_id)
